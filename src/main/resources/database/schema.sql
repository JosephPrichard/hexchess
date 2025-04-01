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
    playedOn TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    winElo NUMERIC,
    loseElo NUMERIC,
    moveList JSONB NOT NULL);

CREATE TABLE IF NOT EXISTS challenges (
    challengerId BIGINT NOT NULL,
    challengeeId BIGINT NOT NULL,
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

-- Utility function to calculate the probability of a win used in the elo formula within the update stats transaction
CREATE OR REPLACE FUNCTION probabilityWins(IN elo1 NUMERIC, IN elo2 NUMERIC)
    RETURNS NUMERIC
    LANGUAGE plpgsql
AS $$
BEGIN
    RETURN 1.0 / (1.0 + POWER(10, (elo1 - elo2) / 400.0));
END $$;

-- Transaction to calculate the new stats of a winner and loser of a game, returning the new stats of each player
CREATE OR REPLACE PROCEDURE updateStats(
    IN winId BIGINT,
    IN loseId BIGINT,
    OUT winEloNext NUMERIC,
    OUT loseEloNext NUMERIC
)
    LANGUAGE plpgsql
AS $$
DECLARE
    winElo NUMERIC;
    loseElo NUMERIC;
BEGIN
    SELECT elo INTO winElo FROM users WHERE id = winId;
    SELECT elo INTO loseElo FROM users WHERE id = loseId;

    winEloNext = winElo + (30 * (1 - probabilityWins(loseElo, winElo)));
    loseEloNext = loseElo + ((30 * probabilityWins(winElo, loseElo)) * -1);

    UPDATE users
    SET elo = winEloNext, wins = wins + 1, highestElo = GREATEST(highestElo, winEloNext)
    WHERE id = winId;

    UPDATE users
    SET elo = loseEloNext, losses = losses + 1
    WHERE id = loseId;
END $$;
END;

-- Insert the base values for a zero initialized schema
BEGIN; INSERT INTO users_metadata (id, count) VALUES (1, 0); END;

-- Trigger to keep the user metadata up to date
CREATE OR REPLACE FUNCTION increment_users_metadata()
    RETURNS TRIGGER AS $$
BEGIN
    UPDATE users_metadata
    SET count = count + 1
    WHERE id = NEW.id;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_increment_users_metadata_on_insert
    AFTER INSERT ON users
    FOR EACH ROW
    EXECUTE FUNCTION increment_users_metadata();