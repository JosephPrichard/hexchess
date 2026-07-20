-- +goose up
CREATE TABLE redis_queue_metrics (
    event_id UUID PRIMARY KEY,
    group_id UUID,
    stream_name TEXT NOT NULL,
    consumed_on TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_on TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_redis_queue_metrics_group_id
    ON redis_queue_metrics (group_id);

-- +goose down
DROP INDEX IF EXISTS idx_redis_queue_metrics_group_id;

DROP TABLE IF EXISTS redis_queue_metrics;