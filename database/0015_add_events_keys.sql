-- +goose up
CREATE TABLE events (
    id UUID PRIMARY KEY,
    data BYTEA NOT NULL,
    consumed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose down
DROP TABLE IF EXISTS events;