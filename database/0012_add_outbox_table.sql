-- +goose up
CREATE TYPE queue_type_enum AS ENUM (
    'TOURNAMENT_ADVANCE_EVENT',
    'TOURNAMENT_CREATE_MATCHES_EVENT'
);

CREATE TABLE event_queue (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type queue_type_enum NOT NULL,
    data BYTEA NOT NULL,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_on TIMESTAMP WITH TIME ZONE,
    scheduled_on TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_event_queue
    ON event_queue (processed_on, type, scheduled_on);

-- +goose down
DROP INDEX IF EXISTS idx_event_queue;
DROP TABLE IF EXISTS event_queue;
DROP TYPE IF EXISTS queue_type_enum;