-- +goose up
ALTER TABLE user_mode_elos
    ADD COLUMN draws INT NOT NULL DEFAULT 0;

-- +goose down
ALTER TABLE user_mode_elos
    DROP COLUMN IF EXISTS draws;