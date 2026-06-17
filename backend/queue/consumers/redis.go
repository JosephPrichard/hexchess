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

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	ctx   context.Context
	redis redis.UniversalClient

	streamKey     string
	consumerGroup string
	concurrency   int64
	partitionKeys []string
	maxEvents     uint64

	fn ConsumeFunc

	waitGroup sync.WaitGroup
}

func (c *RedisConsumer) Consume() error {
	consumerID := uuid.NewString()

	var streams []string
	if len(c.partitionKeys) > 0 {
		for _, partition := range c.partitionKeys {
			streams = append(streams, queue.FmtStreamKey(c.streamKey, partition))
		}
	} else {
		streams = append(streams, c.streamKey)
	}

	for _, stream := range streams {
		if err := c.redis.XGroupCreateMkStream(c.ctx, stream, c.consumerGroup, "0").Err(); err != nil {
			slog.WarnContext(c.ctx, "create stream consumer group", "error", err, "stream", stream, "consumerGroup", c.consumerGroup)
		}
	}

	for range len(streams) {
		// > means get unconsumed messages only. we need to get unconsumed messages from all partitions in this consumer
		streams = append(streams, ">")
	}

	for i := uint64(0); ; i++ {
		if i >= c.maxEvents && c.maxEvents != 0 {
			return nil
		}

		slog.Info("redis stream consumer read operation", "consumerID", consumerID, "streams", streams)

		xArgs := &redis.XReadGroupArgs{
			Group:    c.consumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    c.concurrency,
		}
		entries, err := c.redis.XReadGroup(c.ctx, xArgs).Result()
		if errors.Is(err, context.Canceled) {
			slog.Info("context cancelled, exiting finish event loop")
			return nil
		} else if err != nil {
			slog.Error("failed to read from finish redis stream", "error", err)
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
