package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type ConsumeFunc func(ctx context.Context, bytes []byte) error

type StreamConsumer struct {
	streamKey     string
	consumerGroup string
	pollCount     int64
	partitionKeys []string
	maxEvents     uint64
	blockDuration time.Duration

	// allows receiving cancellation signals (multiple partition consumers will share the cancellation)
	ctx context.Context
	// connects to redis to poll event streams and a database to insert into metadata tables
	redis redis.UniversalClient
	// an implementation for consuming a single event
	consumeFunc ConsumeFunc
	// send a signal on each successfully consumed message to allow determinism (CSP)
	ackChan chan redis.XMessage
}

type StreamConfig struct {
	StreamKey     string        `json:"streamKey"`     // (required) the `parent` stream name for each partition
	ConsumerGroup string        `json:"consumerGroup"` // (required) prevents multiple server nodes from receiving duplicate events
	PollCount     int64         `json:"pollCount"`     // (required) the number of max number messages received in poll attempt. each event is handled on a separate goroutine.
	PartitionKeys []string      `json:"partitionKeys"` // (required) specifies the substreams for a stream to be split into to enable sharding. an empty list will provide no streams
	BlockDuration time.Duration `json:"blockDuration"` // (optional) species the number of time before an XReadGroup will try again to receive messages

	// allows receiving cancellation signals
	Context context.Context

	Redis     redis.UniversalClient `json:"-"`
	ConsumeFn ConsumeFunc           `json:"-"`
	AckChan   chan redis.XMessage   `json:"-"`
}

func NewStreamConsumer(config StreamConfig) *StreamConsumer {
	slog.Info("created redis stream consumer", "config", config)

	return &StreamConsumer{
		streamKey:     config.StreamKey,
		consumerGroup: config.ConsumerGroup,
		pollCount:     config.PollCount,
		partitionKeys: config.PartitionKeys,
		blockDuration: config.BlockDuration,

		ctx:         config.Context,
		redis:       config.Redis,
		consumeFunc: config.ConsumeFn,
		ackChan:     config.AckChan,
	}
}

func (consumer *StreamConsumer) Consume() {
	var wg sync.WaitGroup
	for _, partitionKey := range consumer.partitionKeys {
		wg.Go(func() {
			consumer.consumePartition(partitionKey)
		})
	}
	wg.Wait()

	slog.Info("redis stream consumer exited", "consumer", consumer.streamKey)
}

func (consumer *StreamConsumer) consumePartition(partitionKey string) {
	consumerID := uuid.NewString()

	stream := cache.FmtStreamKey(consumer.streamKey, partitionKey)
	streams := []string{stream, ">"}

	// consumer groups must be created in redis before we consume - they must be provided as constant to consumers of the same stream name to prevent duplicate events
	err := consumer.redis.XGroupCreateMkStream(consumer.ctx, stream, consumer.consumerGroup, "0").Err()
	slog.Warn("create stream consumer group", "error", err, "stream", stream, "consumerGroup", consumer.consumerGroup)

	var wg sync.WaitGroup

	for {
		ctx := slogutil.TracedContext(consumer.ctx, uuid.NewString())

		xArgs := &redis.XReadGroupArgs{
			Group:    consumer.consumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    consumer.pollCount,
			Block:    consumer.blockDuration,
		}
		entries, err := consumer.redis.XReadGroup(ctx, xArgs).Result()
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "stream consumer error: context cancelled, exiting consume partition loop", "stream", stream)
			return
		} else if err != nil {
			slog.ErrorContext(ctx, "stream consumer error: failed to read from redis stream", "error", err, "stream", stream)
			continue
		}

		start := time.Now()

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				wg.Go(func() {
					consumer.handleXReadMessage(ctx, msg)
				})
			}
		}

		wg.Wait()

		slog.InfoContext(ctx, "finished redis stream consume operation", "stream", stream, "timeTaken", time.Since(start).String())
	}
}

func (consumer *StreamConsumer) handleXReadMessage(ctx context.Context, msg redis.XMessage) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "panic: handle redis stream event", "err", r, "streamKey", consumer.streamKey, "stack", string(debug.Stack()))
		}
	}()

	// send the acknowledgement if data is invalid OR message succeeds, retry otherwise
	anyData := msg.Values["data"]
	data, ok := anyData.(string)
	if !ok {
		slog.ErrorContext(ctx, "failed to xread message, data field is incorrect type", "type", fmt.Sprintf("%T", anyData))
		return
	}

	consumedOn := time.Now()

	err := consumer.consumeFunc(ctx, []byte(data))
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle redis consumer event", "error", err)
	}
	if errutil.IsType[NonRetryableQueueError](err) {
		slog.WarnContext(ctx, "discarding non retryable redis streams event", "error", err)
		return
	}

	if err := consumer.redis.XAck(ctx, consumer.streamKey, consumer.consumerGroup, msg.ID).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to send xack for stream message", "error", err)
		return
	}
	if consumer.ackChan != nil {
		consumer.ackChan <- msg
	}

	slog.InfoContext(ctx, "finished handling redis stream event", "stream", consumer.streamKey, "timeTaken", time.Since(consumedOn).String())
}
