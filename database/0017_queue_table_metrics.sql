-- +goose up
ALTER TABLE event_queue ADD COLUMN group_id UUID;
ALTER TABLE event_queue ADD COLUMN consumed_on TIMESTAMPTZ;

CREATE INDEX idx_event_queue_group_id
    ON event_queue (group_id);

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
ALTER TABLE event_queue DROP COLUMN IF EXISTS group_id;
ALTER TABLE event_queue DROP COLUMN IF EXISTS consumed_on;

DROP INDEX IF EXISTS idx_event_queue_group_id;
DROP INDEX IF EXISTS idx_redis_queue_metrics_group_id;

DROP TABLE IF EXISTS redis_queue_metrics;