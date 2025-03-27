BEGIN;
-- Create extensions.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create tables.
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR NOT NULL,
    username VARCHAR NOT NULL,
    country VARCHAR,
    elo NUMERIC NOT NULL,
    highestElo NUMERIC NOT NULL,
    wins INTEGER NOT NULL,
    losses INTEGER NOT NULL,
    bio VARCHAR NOT NULL DEFAULT '',
    joinedOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    password VARCHAR NOT NULL,
    salt VARCHAR NOT NULL,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS users_metadata (
    id NUMERIC,
    count INTEGER,
    PRIMARY KEY (id));

CREATE TABLE IF NOT EXISTS game_histories (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    whiteId VARCHAR NOT NULL,
    blackId VARCHAR NOT NULL,
    result INTEGER NOT NULL,
    data JSON NOT NULL,
    playedOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    winElo NUMERIC,
    loseElo NUMERIC);

CREATE TABLE IF NOT EXISTS challenges (
    challengerId VARCHAR NOT NULL,
    challengeeId VARCHAR NOT NULL,
    madeOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (challengerId, challengeeId));

-- Create indices.
CREATE INDEX IF NOT EXISTS idxTrgmUsername ON users USING GIST (username gist_trgm_ops);
CREATE INDEX IF NOT EXISTS idxUsername ON users(username);
CREATE INDEX IF NOT EXISTS idxElo ON users(elo);
CREATE UNIQUE INDEX idxUniqueUsername ON users (UPPER(username));

CREATE INDEX IF NOT EXISTS idxWhiteId ON game_histories(whiteId, id);
CREATE INDEX IF NOT EXISTS idxBlackId ON game_histories(blackId, id);
CREATE INDEX IF NOT EXISTS idxBothIds ON game_histories(whiteId, blackId, id);

CREATE INDEX IF NOT EXISTS idxChallengee ON challenges(challengeeId, madeOn);
CREATE INDEX IF NOT EXISTS idxChallengee ON challenges(challengerId, madeOn);
ALTER TABLE challenges ADD FOREIGN KEY(challengerId) REFERENCES users(id);
ALTER TABLE challenges ADD FOREIGN KEY(challengeeId) REFERENCES users(id);
END;

BEGIN; INSERT INTO users_metadata (id, count) VALUES (1, 0); END;