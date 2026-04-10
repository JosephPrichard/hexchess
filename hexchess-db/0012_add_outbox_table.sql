-- +goose up
CREATE TYPE outbox_queue_type_enum AS ENUM (
    'TOURNAMENT_ADVANCE_EVENT',
    'TOURNAMENT_CREATE_MATCHES_EVENT',
    'TOURNAMENT_SCHEDULED_EVENT'
);

CREATE TABLE outbox_queue (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type outbox_queue_type_enum NOT NULL,
    data BYTEA NOT NULL,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_on TIMESTAMP WITH TIME ZONE,
    scheduled_on TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_outbox_queue
    ON outbox_queue (processed_on, type, scheduled_on);

-- +goose down
DROP INDEX IF EXISTS idx_outbox_queue;
DROP TABLE IF EXISTS outbox_queue;
DROP TYPE IF EXISTS outbox_queue_type_enum;