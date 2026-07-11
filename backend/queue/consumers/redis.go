package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/queue"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/timeutil"
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
	PollCount int64 `json:"pollCount"`
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
	redis      redis.UniversalClient
	inserter   RedisEventResultInserter
	dispatcher async.Dispatcher
	// an implementation for consuming a single event
	consumeFunc ConsumeFunc
}

func (consumer *RedisConsumer) Consume() {
	if consumer.dispatcher == nil {
		consumer.dispatcher = async.AsyncDispatcher{}
	}

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
	metrics := &RedisMetricCollector{inserter: consumer.inserter, dispatcher: consumer.dispatcher}

	for i := uint64(0); ; i++ {
		if i >= consumer.MaxEvents && consumer.cancel != nil {
			consumer.cancel()
		}

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
			slog.InfoContext(ctx, "context cancelled, exiting consume partition loop", "stream", consumer.StreamKey)
			return
		} else if err != nil {
			slog.ErrorContext(ctx, "failed to read from redis stream", "error", err, "stream", consumer.StreamKey)
			continue
		}

		start := time.Now()

		for _, entry := range entries {
			for _, msg := range entry.Messages {
				waitGroup.Go(func() {
					consumer.handleXReadMessage(ctx, metrics, msg)
				})
			}
		}

		waitGroup.Wait()
		metrics.Persist()

		slog.InfoContext(ctx, "redis stream consume operation", "stream", stream, "timeTaken", time.Since(start).String())
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

	metrics.Collect(RedisEventMetric{
		StreamName:  consumer.StreamKey,
		GroupID:     extractUUID(ctx, msg, "groupId"),
		EventID:     extractUUID(ctx, msg, "eventId"),
		ConsumedOn:  consumedOn,
		ProcessedOn: time.Now(),
	})

	slog.InfoContext(ctx, "redis stream consume operation", "stream", consumer.StreamKey, "timeTaken", time.Since(consumedOn).String())
}

type RedisEventResultInserter interface {
	InsertRedisEventMetas(ctx context.Context, arg []sqlc.InsertRedisEventMetasParams) *sqlc.InsertRedisEventMetasBatchResults
}

type RedisEventMetric struct {
	StreamName  string
	GroupID     uuid.UUID
	EventID     uuid.UUID
	ConsumedOn  time.Time
	ProcessedOn time.Time
}

type RedisMetricCollector struct {
	lock    sync.Mutex
	metrics []RedisEventMetric

	inserter   RedisEventResultInserter
	dispatcher async.Dispatcher
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

	metricRows := make([]sqlc.InsertRedisEventMetasParams, 0, len(c.metrics))

	for _, metric := range c.metrics {
		metricRows = append(metricRows, sqlc.InsertRedisEventMetasParams{
			StreamName:  metric.StreamName,
			EventId:     pgtype.UUID{Bytes: metric.EventID, Valid: true},
			GroupId:     pgtype.UUID{Bytes: metric.GroupID, Valid: true},
			ConsumedOn:  pgtype.Timestamptz{Time: metric.ConsumedOn, Valid: true},
			ProcessedOn: pgtype.Timestamptz{Time: metric.ProcessedOn, Valid: true},
		})
	}

	c.metrics = c.metrics[:0]

	c.dispatcher.Go(func() { c.insertRedisMetrics(metricRows) })
}

const InsertRedisEventMetricMaxRetries = 3

func (c *RedisMetricCollector) insertRedisMetrics(metrics []sqlc.InsertRedisEventMetasParams) {
	var batchErr error
	maxRetries := InsertRedisEventMetricMaxRetries

	for i := range maxRetries {
		batchErr = nil
		// metric insertions fail individually, it is safe to retry the entire batch if any fails because each insert is idempotent
		c.inserter.InsertRedisEventMetas(context.Background(), metrics).Exec(func(index int, err error) {
			batchErr = errors.Join(batchErr, err)
		})
		if batchErr == nil {
			return
		}
		slog.Warn("failed to insert redis event metric", "error", batchErr, "attempt", i)
		timeutil.Sleep(i, 2, 200*time.Millisecond)
	}

	slog.Error("exhausted retries while inserting redis event metric", "error", batchErr, "maxRetries", maxRetries)
}
