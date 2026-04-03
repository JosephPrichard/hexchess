-- +goose up
CREATE TYPE outbox_queue_type_enum AS ENUM (
    'TOURNAMENT_ADVANCE_EVENT'
);

CREATE TABLE outbox_queue (
    id SERIAL PRIMARY KEY,
    type outbox_queue_type_enum NOT NULL,
    data BYTEA NOT NULL,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_on TIMESTAMP WITH TIME ZONE
);

-- +goose down
DROP TABLE IF EXISTS outbox_queue;
DROP TYPE IF EXISTS outbox_queue_type_enum;