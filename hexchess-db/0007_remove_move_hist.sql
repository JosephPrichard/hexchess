-- +goose up
ALTER TABLE replays DROP COLUMN move_history;

-- +goose Down
ALTER TABLE replays
    ADD COLUMN move_history BYTEA;