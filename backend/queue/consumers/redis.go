package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-lib/errutil"
	"hexchess-lib/logutil"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/queue"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	// (required) the `parent` stream name for each partition
	StreamKey string `json:"streamKey"`
	// (required) prevents multiple server nodes from receiving duplicate events
	ConsumerGroup string `json:"consumerGroup"`
	// (required) the number of max number messages received in poll attempt. each event is handled on a seperate goroutine.
	PollCount int64 `json:"concurrency"`
	// (required) specifies the substreams for a stream to be split into to enable sharding. an empty list will provide no streams
	PartitionKeys []string `json:"partitionKeys"`
	// (optional) change the behavior of the consumer for tests
	MaxEvents     uint64        `json:"maxEvents"`
	BlockDuration time.Duration `json:"blockDuration"`
}

type RedisConsumer struct {
	RedisConfig
	// allows receiving and sending cancellation signals (multiple partition consumers will share the cancellation)
	ctx    context.Context
	cancel context.CancelFunc
	// connects to redis to poll event streams and a database to insert into metadata tables
	redis    redis.UniversalClient
	inserter RedisEventResultInserter
	// an implementation for consuming a single event
	consumeFunc ConsumeFunc
}

func (consumer *RedisConsumer) Consume() {
	var wg sync.WaitGroup
	for _, partitionKey := range consumer.PartitionKeys {
		wg.Go(func() {
			consumer.ConsumePartition(partitionKey)
		})
	}
	wg.Wait()

	slog.Info("redis stream consumer exiting", "consumer", consumer)
}

func (consumer *RedisConsumer) ConsumePartition(partitionKey string) {
	consumerID := uuid.NewString()

	stream := queue.FmtStreamKey(consumer.StreamKey, partitionKey)
	streams := []string{stream, ">"}

	err := consumer.redis.XGroupCreateMkStream(consumer.ctx, stream, consumer.ConsumerGroup, "0").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		slog.Error("create stream consumer group", "error", err, "stream", stream, "consumerGroup", consumer.ConsumerGroup)
	}

	var waitGroup sync.WaitGroup
	metrics := &RedisMetricCollector{inserter: consumer.inserter}

	for i := uint64(0); ; i++ {
		if i >= consumer.MaxEvents && consumer.cancel != nil {
			consumer.cancel()
		}

		start := time.Now()
		ctx := context.WithValue(consumer.ctx, logutil.Trace, uuid.NewString())

		xArgs := &redis.XReadGroupArgs{
			Group:    consumer.ConsumerGroup,
			Consumer: consumerID,
			Streams:  streams,
			Count:    consumer.PollCount,
			Block:    consumer.BlockDuration,
		}
		entries, err := consumer.redis.XReadGroup(ctx, xArgs).Result()
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
					consumer.handleXReadMessage(ctx, metrics, msg)
				})
			}
		}

		waitGroup.Wait()
		metrics.Persist()

		slog.InfoContext(ctx, "end redis stream consume operation", "stream", stream, "timeTaken", time.Since(start).String())
	}
}

func (consumer *RedisConsumer) handleXReadMessage(ctx context.Context, metrics *RedisMetricCollector, msg redis.XMessage) {
	// send the acknowledgement if data is invalid OR message succeeds, retry otherwise
	anyData := msg.Values["data"]
	data, ok := anyData.(string)
	if !ok {
		slog.ErrorContext(ctx, "failed to xread message, data field is incorrect type", "type", fmt.Sprintf("%T", anyData))
		return
	}

	anyGroupID := msg.Values["groupId"]
	groupIDStr, _ := anyGroupID.(string)

	consumedOn := time.Now()

	err := consumer.consumeFunc(ctx, []byte(data))
	if err != nil {
		slog.ErrorContext(ctx, "failed to handle redis consumer event", "error", err)
	}
	if errutil.IsType[NonRetryableQueueError](err) {
		return
	}
	err = consumer.redis.XAck(ctx, consumer.StreamKey, consumer.ConsumerGroup, msg.ID).Err()
	if err != nil {
		slog.ErrorContext(ctx, "failed to send xack for stream message", "error", err)
		return
	}

	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		slog.WarnContext(ctx, "received invalid group id on stream", "error", err)
		groupID = uuid.New()
	}
	metrics.Collect(RedisEventMetric{
		StreamName:  consumer.StreamKey,
		GroupID:     groupID,
		ConsumedOn:  consumedOn,
		ProcessedOn: time.Now(),
	})
}

type RedisEventResultInserter interface {
	InsertRedisEventMetas(ctx context.Context, arg []sqlc.InsertRedisEventMetasParams) *sqlc.InsertRedisEventMetasBatchResults
}

type RedisEventMetric struct {
	StreamName  string
	GroupID     uuid.UUID
	ConsumedOn  time.Time
	ProcessedOn time.Time
}

type RedisMetricCollector struct {
	lock    sync.Mutex
	metrics []RedisEventMetric

	inserter RedisEventResultInserter
}

func (c *RedisMetricCollector) Collect(metric RedisEventMetric) {
	c.lock.Lock()
	c.metrics = append(c.metrics, metric)
	c.lock.Unlock()
}

func (c *RedisMetricCollector) Persist() {
	if len(c.metrics) == 0 {
		return
	}

	ctx := context.Background()

	rows := make([]sqlc.InsertRedisEventMetasParams, 0, len(c.metrics))

	for _, metric := range c.metrics {
		rows = append(rows, sqlc.InsertRedisEventMetasParams{
			GroupId:     pgtype.UUID{Bytes: metric.GroupID, Valid: true},
			StreamName:  metric.StreamName,
			ConsumedOn:  pgtype.Timestamptz{Time: metric.ConsumedOn, Valid: true},
			ProcessedOn: pgtype.Timestamptz{Time: metric.ConsumedOn, Valid: true},
		})
	}
	c.metrics = c.metrics[:0]

	// note: fire and forget, we'd prefer to store these metrics but its not worth retrying if storage fails
	go func() {
		if c.inserter != nil {
			return
		}
		c.inserter.InsertRedisEventMetas(ctx, rows).Exec(func(index int, err error) {
			if err != nil {
				slog.Error("failed to ship redis event metadata", "error", err)
			}
		})
	}()
}
