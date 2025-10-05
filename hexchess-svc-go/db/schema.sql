-- Create the database schema
BEGIN;
-- Create extensions.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create tables.
CREATE TABLE IF NOT EXISTS users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username VARCHAR NOT NULL,
    country VARCHAR,
    elo NUMERIC NOT NULL,
    highestElo NUMERIC NOT NULL,
    wins INTEGER NOT NULL,
    losses INTEGER NOT NULL,
    bio VARCHAR NOT NULL DEFAULT '',
    joinedOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    password VARCHAR NOT NULL,
    salt VARCHAR NOT NULL);

CREATE TABLE IF NOT EXISTS users_metadata (
    id BIGINT,
    count INTEGER,
    PRIMARY KEY (id));

CREATE TABLE IF NOT EXISTS replays (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    whiteId BIGINT NOT NULL,
    blackId BIGINT NOT NULL,
    result INTEGER NOT NULL,
    cause INTEGER NOT NULL,
    playedOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    winElo NUMERIC,
    loseElo NUMERIC,
    moveList JSONB NOT NULL);

CREATE TABLE IF NOT EXISTS challenges (
    challengerId BIGINT NOT NULL,
    challengeeId BIGINT NOT NULL,
    timeControl VARCHAR NOT NULL,
    startColor VARCHAR NOT NULL, -- from challenger's perspective
    madeOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (challengerId, challengeeId));

-- Create indices.
CREATE INDEX IF NOT EXISTS idxTrgmUsername ON users USING GIST (username gist_trgm_ops);
CREATE INDEX IF NOT EXISTS idxUsername ON users(username);
CREATE INDEX IF NOT EXISTS idxElo ON users(elo);
CREATE UNIQUE INDEX IF NOT EXISTS idxUniqueUsername ON users (UPPER(username));

CREATE INDEX IF NOT EXISTS idxWhiteId ON replays(whiteId, id);
CREATE INDEX IF NOT EXISTS idxBlackId ON replays(blackId, id);
CREATE INDEX IF NOT EXISTS idxBothIds ON replays(whiteId, blackId, id);
ALTER TABLE replays ADD FOREIGN KEY(whiteId) REFERENCES users(id);
ALTER TABLE replays ADD FOREIGN KEY(blackId) REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idxChallengee ON challenges(challengeeId, madeOn);
CREATE INDEX IF NOT EXISTS idxChallengee ON challenges(challengerId, madeOn);
ALTER TABLE challenges ADD FOREIGN KEY(challengerId) REFERENCES users(id);
ALTER TABLE challenges ADD FOREIGN KEY(challengeeId) REFERENCES users(id);
END;

-- Insert the base values for a zero initialized schema
BEGIN; INSERT INTO users_metadata (id, count) VALUES (1, 0); END;