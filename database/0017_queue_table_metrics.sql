-- +goose up
ALTER TABLE event_queue ADD COLUMN group_id UUID;
ALTER TABLE event_queue ADD COLUMN consumed_on TIMESTAMPTZ;

CREATE INDEX idx_event_queue_group_id
    ON event_queue (group_id);

CREATE TABLE redis_queue_metadata (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id UUID,
    stream_name TEXT NOT NULL,
    consumed_on TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_on TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_redis_queue_metadata_group_id
    ON event_queue (group_id);

-- +goose down
ALTER TABLE event_queue DROP COLUMN group_id IF EXISTS;
ALTER TABLE event_queue DROP COLUMN consumed_on IF TIMESTAMPTZ;

DROP TABLE redis_queue_metadata IF EXISTS;

DROP INDEX idx_event_queue_group_id IF EXISTS;
DROP INDEX idx_redis_queue_metadata_group_id IF EXISTS;