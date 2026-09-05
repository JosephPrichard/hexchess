package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/utils/async"
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

	// allows receiving and sending cancellation signals (multiple partition consumers will share the cancellation)
	ctx    context.Context
	cancel context.CancelFunc
	// connects to redis to poll event streams and a database to insert into metadata tables
	redis      redis.UniversalClient
	querier    database.QuerierMutator
	dispatcher async.Dispatcher
	// an implementation for consuming a single event
	consumeFunc ConsumeFunc
}

type StreamConfig struct {
	StreamKey     string        `json:"streamKey"`     // (required) the `parent` stream name for each partition
	ConsumerGroup string        `json:"consumerGroup"` // (required) prevents multiple server nodes from receiving duplicate events
	PollCount     int64         `json:"pollCount"`     // (required) the number of max number messages received in poll attempt. each event is handled on a separate goroutine.
	PartitionKeys []string      `json:"partitionKeys"` // (required) specifies the substreams for a stream to be split into to enable sharding. an empty list will provide no streams
	MaxEvents     uint64        `json:"maxEvents"`     // (opt) change the behavior of the consumer for tests
	BlockDuration time.Duration `json:"blockDuration"`

	Redis          redis.UniversalClient   `json:"-"`
	MetricsQuerier database.QuerierMutator `json:"-"`
	ConsumeFn      ConsumeFunc             `json:"-"`
}

func NewStreamConsumer(config StreamConfig) *StreamConsumer {
	slog.Info("created redis stream consumer", "config", config)

	return &StreamConsumer{
		streamKey:     config.StreamKey,
		consumerGroup: config.ConsumerGroup,
		pollCount:     config.PollCount,
		partitionKeys: config.PartitionKeys,
		maxEvents:     config.MaxEvents,
		blockDuration: config.BlockDuration,

		ctx:         context.Background(),
		redis:       config.Redis,
		querier:     config.MetricsQuerier,
		consumeFunc: config.ConsumeFn,
		dispatcher:  async.AsyncDispatcher{},
	}
}

func (consumer *StreamConsumer) Consume() {
	var wg sync.WaitGroup
	for _, partitionKey := range consumer.partitionKeys {
		wg.Go(func() {
			consumer.ConsumePartition(partitionKey)
		})
	}
	wg.Wait()

	slog.Info("redis stream consumer exiting", "consumer", consumer)
}

func (consumer *StreamConsumer) ConsumePartition(partitionKey string) {
	consumerID := uuid.NewString()

	stream := cache.FmtStreamKey(consumer.streamKey, partitionKey)
	streams := []string{stream, ">"}

	err := consumer.redis.XGroupCreateMkStream(consumer.ctx, stream, consumer.consumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		slog.Error("create stream consumer group", "error", err, "stream", stream, "consumerGroup", consumer.consumerGroup)
	}

	var wg sync.WaitGroup
	metrics := &RedisMetricCollector{querier: consumer.querier, dispatcher: consumer.dispatcher}

	for i := uint64(0); ; i++ {
		if i >= consumer.maxEvents && consumer.cancel != nil {
			consumer.cancel()
		}

		ctx := context.WithValue(consumer.ctx, slogutil.Trace, uuid.NewString())

		xArgs := &redis.XReadGroupArgs{
			Group:    consumer.consumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    consumer.pollCount,
			Block:    consumer.blockDuration,
		}
		entries, err := consumer.redis.XReadGroup(ctx, xArgs).Result()
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "context cancelled, exiting consume partition loop", "stream", consumer.streamKey)
			return
		} else if err != nil {
			slog.ErrorContext(ctx, "failed to read from redis stream", "error", err, "stream", consumer.streamKey)
			continue
		}

		start := time.Now()

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				wg.Go(func() {
					consumer.handleXReadMessage(ctx, metrics, msg)
				})
			}
		}

		wg.Wait()
		metrics.Persist()

		slog.InfoContext(ctx, "finished redis stream consume operation", "stream", stream, "timeTaken", time.Since(start).String())
	}
}

func (consumer *StreamConsumer) handleXReadMessage(ctx context.Context, metrics *RedisMetricCollector, msg redis.XMessage) {
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

	metrics.Collect(RedisEventMetric{
		StreamName:  consumer.streamKey,
		GroupID:     extractUUID(ctx, msg, "groupId"),
		EventID:     extractUUID(ctx, msg, "eventId"),
		ConsumedOn:  consumedOn,
		ProcessedOn: time.Now(),
	})

	slog.InfoContext(ctx, "finished handling redis stream event", "stream", consumer.streamKey, "timeTaken", time.Since(consumedOn).String())
}
