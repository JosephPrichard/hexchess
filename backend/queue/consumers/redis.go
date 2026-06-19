package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/logutil"
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

	streamKey     string
	consumerGroup string
	concurrency   int64
	partitionKeys []string
	maxEvents     uint64
	blockDuration time.Duration

	fn ConsumeFunc

	waitGroup sync.WaitGroup
}

func (c *RedisConsumer) Consume() {
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	if c.cancel == nil {
		c.cancel = func() {}
	}

	var wg sync.WaitGroup
	for _, partitionKey := range c.partitionKeys {
		wg.Go(func() {
			c.ConsumePartition(partitionKey)
		})
	}
	wg.Wait()
	slog.Info("redis stream consumer exiting", "consumer", c)
}

func (c *RedisConsumer) ConsumePartition(partitionKey string) {
	consumerID := uuid.NewString()

	stream := queue.FmtStreamKey(c.streamKey, partitionKey)
	streams := []string{stream, ">"}

	err := c.redis.XGroupCreateMkStream(c.ctx, stream, c.consumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		slog.Error("create stream consumer group", "error", err, "consumerID", consumerID, "stream", stream, "consumerGroup", c.consumerGroup)
	}

	for i := uint64(0); ; i++ {
		if i >= c.maxEvents && c.maxEvents != 0 {
			c.cancel()
		}

		slog.Info("redis stream consumer read operation", "consumerID", consumerID, "streams", streams)

		xArgs := &redis.XReadGroupArgs{
			Group:    c.consumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    c.concurrency,
			Block:    c.blockDuration,
		}
		entries, err := c.redis.XReadGroup(c.ctx, xArgs).Result()
		if errors.Is(err, context.Canceled) {
			slog.Info("context cancelled, exiting consume partition loop", "consumerID", consumerID, "streams", streams)
			return
		} else if err != nil {
			slog.Error("failed to read from redis stream", "error", err, "consumerID", consumerID, "streams", streams)
			continue
		}

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				c.waitGroup.Go(func() {
					slog.Info("redis stream consumer read message", "consumerID", consumerID, "stream", entry.Stream, "id", msg.ID)
					c.handleXReadMessage(msg)
				})
			}
		}

		c.waitGroup.Wait()
	}
}

func (c *RedisConsumer) handleXReadMessage(msg redis.XMessage) {
	ctx := context.WithValue(c.ctx, logutil.Trace, uuid.NewString())

	// send the acknowledgement if data is invalid OR message succeeds, retry otherwise
	anyData := msg.Values["data"]
	data, ok := anyData.(string)
	if !ok {
		slog.ErrorContext(ctx, "failed to xread message, data field is incorrect type", "type", fmt.Sprintf("%T", anyData))
		return
	}
	err := c.fn(ctx, []byte(data))
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle redis consumer event", "error", err)
	}
	if errutil.IsType[NonRetryableQueueError](err) {
		return
	}
	if err := c.redis.XAck(c.ctx, c.streamKey, c.consumerGroup, msg.ID).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to send xack", "id", msg.ID, "error", err)
	} else {
		slog.InfoContext(ctx, "sent xack", "id", msg.ID)
	}
}
