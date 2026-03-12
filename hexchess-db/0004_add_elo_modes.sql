-- +goose up
CREATE TYPE mode_enum AS ENUM (
    'TIMED_1+0',
    'TIMED_3+2',
    'TIMED_15+10',
    'CORRESPONDENCE_1',
    'CORRESPONDENCE_7',
    'CORRESPONDENCE_14',
    'TIMED_5+0'
);

CREATE TABLE user_mode_elos (
    user_id BIGINT NOT NULL,
    mode mode_enum NOT NULL,
    elo FLOAT8 NOT NULL,
    highest_elo FLOAT8 NOT NULL,
    wins INT NOT NULL DEFAULT 0,
    losses INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, mode),
    CONSTRAINT user_mode_elos_userid_fkey
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE
);

CREATE INDEX idx_user_mode_elos_userid
    ON user_mode_elos (user_id);

ALTER TABLE users
    ALTER COLUMN google_account_id SET DEFAULT NULL,
    DROP COLUMN IF EXISTS elo,
    DROP COLUMN IF EXISTS highest_elo,
    DROP COLUMN IF EXISTS start_elo,
    DROP COLUMN IF EXISTS wins,
    DROP COLUMN IF EXISTS losses;

ALTER TABLE challenges
    DROP CONSTRAINT time_control_check,
    DROP COLUMN IF EXISTS time_control;

ALTER TABLE challenges
    ADD COLUMN mode mode_enum NOT NULL;

ALTER TABLE replays
    DROP CONSTRAINT mode_check;

ALTER TABLE replays
    ALTER COLUMN mode TYPE mode_enum USING mode::text::mode_enum;

-- +goose down

-- Revert replays.mode to 'text' (or previous type) and restore constraint
ALTER TABLE replays
    ALTER COLUMN mode TYPE TEXT USING mode::text;

ALTER TABLE replays
    ADD CONSTRAINT mode_check
        CHECK (mode IN (
            'TIMED_1+0',
            'TIMED_3+2',
            'TIMED_15+10',
            'CORRESPONDENCE_1',
            'CORRESPONDENCE_7',
            'CORRESPONDENCE_14',
            'TIMED_5+0'
        ));

-- Revert challenges changes
ALTER TABLE challenges
    DROP COLUMN IF EXISTS mode;

ALTER TABLE challenges
    ADD COLUMN time_control TEXT;

ALTER TABLE challenges
    ADD CONSTRAINT time_control_check
        CHECK (time_control IN (
            'TIMED_1+0',
            'TIMED_3+2',
            'TIMED_15+10',
            'CORRESPONDENCE_1',
            'CORRESPONDENCE_7',
            'CORRESPONDENCE_14',
            'TIMED_5+0'
        ));

-- Restore users columns
ALTER TABLE users
    ADD COLUMN elo FLOAT8,
    ADD COLUMN highest_elo FLOAT8,
    ADD COLUMN start_elo FLOAT8,
    ADD COLUMN wins INT,
    ADD COLUMN losses INT;

ALTER TABLE users
    ALTER COLUMN google_account_id DROP DEFAULT;

-- Drop index and table before dropping enum
DROP INDEX IF EXISTS idx_user_mode_elos_userid;

DROP TABLE IF EXISTS user_mode_elos;

DROP TYPE IF EXISTS mode_enum;