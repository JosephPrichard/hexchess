-- +goose up
CREATE TYPE tournament_status_enum AS ENUM (
    'LOBBY',
    'IN_PROGRESS',
    'FINISHED'
);

CREATE TABLE tournaments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tkey UUID NOT NULL UNIQUE,
    name TEXT NOT NULL,
    rounds INT NOT NULL,
    status tournament_status_enum NOT NULL,
    scheduled_on TIMESTAMP WITH TIME ZONE,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT NOT NULL REFERENCES users(id),
    updated_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    mode mode_enum NOT NULL
);

CREATE TABLE tournament_participants (
    tournament_key UUID NOT NULL REFERENCES tournaments(tkey),
    user_id BIGINT NOT NULL REFERENCES users(id),
    joined_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT tournament_participants_pkey PRIMARY KEY (tournament_key, user_id)
);

CREATE TABLE tournament_matches (
    ordering BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tournament_key UUID NOT NULL REFERENCES tournaments(tkey),
    round INT NOT NULL,
    game_id TEXT NOT NULL UNIQUE,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tournament_participants_userid
    ON tournament_participants (user_id);

CREATE INDEX idx_tournament_participants_tournament_key
    ON tournament_participants (tournament_key);

CREATE INDEX idx_tournament_matches_tournament_key
    ON tournament_matches (tournament_key);

-- +goose down
DROP TABLE IF EXISTS tournament_matches;
DROP TABLE IF EXISTS tournament_participants;
DROP TABLE IF EXISTS tournaments;
DROP TYPE IF EXISTS tournament_status_enum;