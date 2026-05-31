-- +goose up
CREATE TABLE games_metadata (
    ordering BIGINT UNIQUE NOT NULL,
    game_id TEXT PRIMARY KEY,
    mode mode_enum NOT NULL,
    white_id BIGINT,
    black_id BIGINT,
    updated_on TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE VIEW games_metadata_count AS
SELECT COUNT(*) AS total
FROM games_metadata;

CREATE SEQUENCE games_metadata_ordering_seq START 1;

-- +goose down
DROP SEQUENCE IF EXISTS games_metadata_ordering_seq;
DROP VIEW IF EXISTS games_metadata_count;
DROP TABLE IF EXISTS games_metadata;
