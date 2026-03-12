-- +goose up
ALTER TABLE replays ADD COLUMN game_id TEXT NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX idx_replay_game_id
    ON replays (game_id);
ALTER TABLE replays ALTER COLUMN game_id DROP DEFAULT;

-- +goose down
ALTER TABLE replays DROP COLUMN game_id;
DROP INDEX IF EXISTS idx_replay_game_id;