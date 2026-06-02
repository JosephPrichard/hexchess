package consumers

import (
	"context"
	"fmt"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/logutil"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	ctx   context.Context
	redis redis.UniversalClient

	consumerName  string
	concurrency   int64
	streamKey     string
	consumerGroup string
	maxEvents     uint64

	fn ConsumeFunc

	waitGroup sync.WaitGroup
}

func (c *RedisConsumer) Consume() error {
	consumerID := uuid.NewString()
	streamKey := c.streamKey
	streams := []string{streamKey, ">"}

	err := c.redis.XGroupCreateMkStream(c.ctx, streamKey, c.consumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create stream consumer group: %w", err)
	}

	for i := uint64(0); ; i++ {
		if i >= c.maxEvents && c.maxEvents != 0 {
			return nil
		}

		slog.Info("redis stream consumer read operation", "consumerID", consumerID, "streamKey", streamKey)

		xArgs := &redis.XReadGroupArgs{
			Group:    c.consumerGroup,
			Consumer: consumerID,
			Streams:  streams, // ">" means only undelivered messages
			Count:    c.concurrency,
		}
		entries, err := c.redis.XReadGroup(c.ctx, xArgs).Result()
		switch err {
		case context.Canceled:
			slog.Info("context cancelled, exiting finish event loop")
			return nil
		default:
			slog.Error("failed to read from finish redis stream", "error", err)
			continue
		case nil:
			// handle the event
		}

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				c.waitGroup.Go(func() {
					c.handleXReadMessage(msg)
				})
			}
		}

		c.waitGroup.Wait()
	}
}

func (c *RedisConsumer) handleXReadMessage(msg redis.XMessage) {
	ctx := context.WithValue(c.ctx, logutil.Trace, uuid.NewString())

	// send the acknowledge if data is invalid OR message succeeds, retry otherwise
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
