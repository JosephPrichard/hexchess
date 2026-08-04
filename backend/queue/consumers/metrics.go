package consumers

import (
	"context"
	"errors"
	"hexchess-svc/db"
	"hexchess-svc/db/mutator"

	"hexchess-svc/utils/async"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

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

	querier    db.QuerierMutator
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

	metricRows := make([]mutator.InsertRedisEventMetricsParams, 0, len(c.metrics))

	for _, metric := range c.metrics {
		metricRows = append(metricRows, mutator.InsertRedisEventMetricsParams{
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

func (c *RedisMetricCollector) insertRedisMetrics(metrics []mutator.InsertRedisEventMetricsParams) {
	var batchErr error
	maxRetries := InsertRedisEventMetricMaxRetries

	for i := range maxRetries {
		batchErr = nil
		// metric insertions fail individually, it is safe to retry the entire batch if any fails because each insert is idempotent
		c.querier.InsertRedisEventMetrics(context.Background(), metrics).Exec(func(index int, err error) {
			batchErr = errors.Join(batchErr, err)
		})
		if batchErr == nil {
			return
		}
		slog.Warn("failed to insert redis event metric", "error", batchErr, "attempt", i)

		timeutil.BackoffSleep(i, 2, 200*time.Millisecond)
	}

	slog.Error("exhausted retries while inserting redis event metric", "error", batchErr, "maxRetries", maxRetries)
}
