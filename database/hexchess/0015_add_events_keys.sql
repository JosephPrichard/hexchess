-- +goose up
CREATE TABLE event_keys (
    id UUID PRIMARY KEY,
    data BYTEA NOT NULL,
    consumed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose down
DROP TABLE IF EXISTS event_keys;