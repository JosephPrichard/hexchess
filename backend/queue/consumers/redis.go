package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-lib/errutil"
	"hexchess-lib/logutil"
	"hexchess-svc/queue"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	ctx    context.Context
	cancel context.CancelFunc
	redis  redis.UniversalClient

	StreamKey     string        `json:"streamKey"`
	ConsumerGroup string        `json:"consumerGroup"`
	Concurrency   int64         `json:"concurrency"`
	PartitionKeys []string      `json:"partitionKeys"`
	MaxEvents     uint64        `json:"maxEvents"`
	BlockDuration time.Duration `json:"blockDuration"`

	consumeFunc ConsumeFunc
}

func (c *RedisConsumer) Consume() {
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	if c.cancel == nil {
		c.cancel = func() {}
	}

	var wg sync.WaitGroup
	for _, partitionKey := range c.PartitionKeys {
		wg.Go(func() {
			c.ConsumePartition(partitionKey)
		})
	}
	wg.Wait()
	slog.Info("redis stream consumer exiting", "consumer", c)
}

func (c *RedisConsumer) ConsumePartition(partitionKey string) {
	consumerID := uuid.NewString()

	stream := queue.FmtStreamKey(c.StreamKey, partitionKey)
	streams := []string{stream, ">"}

	err := c.redis.XGroupCreateMkStream(c.ctx, stream, c.ConsumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		slog.Error("create stream consumer group", "error", err, "stream", stream, "consumerGroup", c.ConsumerGroup)
	}

	var waitGroup sync.WaitGroup

	for i := uint64(0); ; i++ {
		if i >= c.MaxEvents && c.MaxEvents != 0 {
			c.cancel()
		}

		start := time.Now()
		ctx := context.WithValue(c.ctx, logutil.Trace, uuid.NewString())

		xArgs := &redis.XReadGroupArgs{
			Group:    c.ConsumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    c.Concurrency,
			Block:    c.BlockDuration,
		}
		entries, err := c.redis.XReadGroup(ctx, xArgs).Result()
		if errors.Is(err, context.Canceled) {
			slog.InfoContext(ctx, "context cancelled, exiting consume partition loop", "stream", stream)
			return
		} else if err != nil {
			slog.ErrorContext(ctx, "failed to read from redis stream", "error", err, "stream", stream)
			continue
		}

		slog.InfoContext(ctx, "begin redis stream consume operation", "stream", stream)

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				waitGroup.Go(func() {
					c.handleXReadMessage(ctx, msg)
				})
			}
		}

		waitGroup.Wait()

		slog.InfoContext(ctx, "end redis stream consume operation", "stream", stream, "timeTaken", time.Since(start).String())
	}
}

func (c *RedisConsumer) handleXReadMessage(ctx context.Context, msg redis.XMessage) {
	// send the acknowledgement if data is invalid OR message succeeds, retry otherwise
	anyData := msg.Values["data"]
	data, ok := anyData.(string)
	if !ok {
		slog.ErrorContext(ctx, "failed to xread message, data field is incorrect type", "type", fmt.Sprintf("%T", anyData))
		return
	}
	err := c.consumeFunc(ctx, []byte(data))
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle redis consumer event", "error", err)
	}
	if errutil.IsType[NonRetryableQueueError](err) {
		return
	}
	if err := c.redis.XAck(c.ctx, c.StreamKey, c.ConsumerGroup, msg.ID).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to send xack for stream message", "error", err)
	}
}
