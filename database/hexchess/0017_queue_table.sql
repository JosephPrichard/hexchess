-- +goose up
ALTER TABLE event_queue ADD COLUMN group_id UUID;
ALTER TABLE event_queue ADD COLUMN consumed_on TIMESTAMPTZ;

CREATE INDEX idx_event_queue_group_id
    ON event_queue (group_id);

-- +goose down
ALTER TABLE event_queue DROP COLUMN IF EXISTS group_id;
ALTER TABLE event_queue DROP COLUMN IF EXISTS consumed_on;

DROP INDEX IF EXISTS idx_event_queue_group_id;