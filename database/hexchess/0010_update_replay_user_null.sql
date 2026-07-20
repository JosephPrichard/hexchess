-- +goose up
ALTER TABLE replays ALTER COLUMN white_id DROP NOT NULL;
ALTER TABLE replays ALTER COLUMN black_id DROP NOT NULL;

-- +goose down
ALTER TABLE replays ALTER COLUMN white_id SET NOT NULL;
ALTER TABLE replays ALTER COLUMN black_id SET NOT NULL;