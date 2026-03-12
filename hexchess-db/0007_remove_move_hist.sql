-- +goose up
ALTER TABLE replays DROP COLUMN move_history;

-- +goose down
ALTER TABLE replays
    ADD COLUMN move_history BYTEA;