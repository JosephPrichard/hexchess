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
	redis *redis.Client

	consumerName  string
	concurrency   int64
	streamKey     string
	consumerGroup string
	maxEvents     uint64

	fn ConsumeFunc

	waitGroup sync.WaitGroup
}

func (q *RedisConsumer) Consume() error {
	consumerID := uuid.NewString()
	streamKey := q.streamKey

	err := q.redis.XGroupCreateMkStream(q.ctx, streamKey, q.consumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		return fmt.Errorf("create stream consumer group: %w", err)
	}

	for i := uint64(0); ; i++ {
		if i >= q.maxEvents && q.maxEvents != 0 {
			return nil
		}

		slog.Info("begin redis stream consumer read operation", "consumerID", consumerID, "streamKey", streamKey)

		xArgs := &redis.XReadGroupArgs{
			Group:    q.consumerGroup,
			Consumer: consumerID,
			Streams:  []string{streamKey, ">"}, // ">" means only undelivered messages
			Count:    q.concurrency,
		}
		entries, err := q.redis.XReadGroup(q.ctx, xArgs).Result()
		switch err {
		case nil:
			// handle the event
		case context.Canceled:
			slog.Info("context cancelled, exiting finish event loop")
			return nil
		default:
			slog.Error("failed to read from finish redis stream", "error", err)
			continue
		}

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				q.waitGroup.Go(func() {
					q.handleXReadMessage(msg)
				})
			}
		}

		q.waitGroup.Wait()
	}
}

func (q *RedisConsumer) handleXReadMessage(msg redis.XMessage) {
	ctx := context.WithValue(q.ctx, logutil.Trace, uuid.NewString())

	acknowledge := func() bool {
		// send the acknowledge if data is invalid OR message succeeds, retry otherwise
		anyData := msg.Values["data"]
		data, ok := anyData.(string)
		if !ok {
			slog.ErrorContext(ctx, "failed to xread message, 'data' field is incorrect type", "type", fmt.Sprintf("%T", anyData))
			return true
		}

		err := q.fn(ctx, []byte(data))
		if err != nil {
			slog.ErrorContext(ctx, "failed to handle redis consumer event", "error", err)
			if errutil.IsType[NonRetryableQueueError](err) {
				return true
			} else {
				return false
			}
		}

		return false
	}()
	if !acknowledge {
		return
	}
	if err := q.redis.XAck(q.ctx, q.streamKey, q.consumerGroup, msg.ID).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to send xack", "id", msg.ID, "error", err)
	} else {
		slog.InfoContext(ctx, "sent xack", "id", msg.ID)
	}
}
