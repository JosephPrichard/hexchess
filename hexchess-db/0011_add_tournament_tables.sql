-- +goose up
CREATE TYPE tournament_status_enum AS ENUM (
    'LOBBY',
    'IN_PROGRESS',
    'FINISHED'
);

CREATE TABLE tournaments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    tournament_key TEXT NOT NULL UNIQUE,
    depth INT NOT NULL,
    status tournament_status_enum NOT NULL,
    scheduled_on TIMESTAMP WITH TIME ZONE,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT NOT NULL REFERENCES users(id),
    updated_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    mode mode_enum NOT NULL
);

CREATE TABLE tournament_participants (
    tournament_id BIGINT NOT NULL REFERENCES tournaments(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    joined_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tournament_matches (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    game_id TEXT UNIQUE,
    tournament_id BIGINT NOT NULL REFERENCES tournaments(id),
    depth INT NOT NULL,
    white_id BIGINT NOT NULL REFERENCES users(id),
    black_id BIGINT NOT NULL REFERENCES users(id),
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose down
DROP TABLE IF EXISTS tournament_matches;
DROP TABLE IF EXISTS tournament_participants;
DROP TABLE IF EXISTS tournaments;
DROP TYPE IF EXISTS tournament_status_enum;