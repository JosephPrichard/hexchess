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
    depth INT NOT NULL,
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
    joined_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tournament_matches (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    game_id UUID NOT NULL UNIQUE,
    tournament_key UUID NOT NULL REFERENCES tournaments(tkey),
    depth INT NOT NULL,
    created_on TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose down
DROP TABLE IF EXISTS tournament_matches;
DROP TABLE IF EXISTS tournament_participants;
DROP TABLE IF EXISTS tournaments;
DROP TYPE IF EXISTS tournament_status_enum;