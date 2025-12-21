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

