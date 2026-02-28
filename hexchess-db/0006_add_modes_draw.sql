-- +goose up
ALTER TABLE user_mode_elos
    ADD COLUMN draws INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE user_mode_elos
    DROP COLUMN IF EXISTS draws;