-- Create the database schema
BEGIN;

-- Create extensions
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create tables
CREATE TABLE IF NOT EXISTS users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username VARCHAR NOT NULL,
    country VARCHAR,
    elo FLOAT8 NOT NULL,
    highest_elo FLOAT8 NOT NULL,
    wins INTEGER NOT NULL,
    losses INTEGER NOT NULL,
    bio VARCHAR NOT NULL DEFAULT '',
    joined_on TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    password VARCHAR NOT NULL,
    salt VARCHAR NOT NULL,
    login_attempts INTEGER NOT NULL DEFAULT 0,
    last_login_attempt TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users_metadata (
    id BIGINT PRIMARY KEY,
    count INTEGER
);

CREATE TABLE IF NOT EXISTS replays (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    white_id BIGINT NOT NULL,
    black_id BIGINT NOT NULL,
    result INTEGER NOT NULL,
    cause INTEGER NOT NULL,
    played_on TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    win_elo FLOAT8 NOT NULL,
    lose_elo FLOAT8 NOT NULL,
    move_list JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS challenges (
    challenger_id BIGINT NOT NULL,
    challengee_id BIGINT NOT NULL,
    time_control VARCHAR NOT NULL,
    start_color VARCHAR NOT NULL, -- from challenger's perspective
    made_on TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (challenger_id, challengee_id)
);

-- Create indices
CREATE INDEX IF NOT EXISTS idx_trgm_username ON users USING GIST (username gist_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_elo ON users(elo);
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_username ON users (UPPER(username));

CREATE INDEX IF NOT EXISTS idx_white_id ON replays(white_id, id);
CREATE INDEX IF NOT EXISTS idx_black_id ON replays(black_id, id);
CREATE INDEX IF NOT EXISTS idx_both_ids ON replays(white_id, black_id, id);
ALTER TABLE replays ADD FOREIGN KEY(white_id) REFERENCES users(id);
ALTER TABLE replays ADD FOREIGN KEY(black_id) REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_challengee ON challenges(challengee_id, made_on);
CREATE INDEX IF NOT EXISTS idx_challenger ON challenges(challenger_id, made_on);
ALTER TABLE challenges ADD FOREIGN KEY(challenger_id) REFERENCES users(id);
ALTER TABLE challenges ADD FOREIGN KEY(challengee_id) REFERENCES users(id);

END;

-- Insert the base values for a zero-initialized schema
BEGIN;
INSERT INTO users_metadata (id, count) VALUES (1, 0);
END;
