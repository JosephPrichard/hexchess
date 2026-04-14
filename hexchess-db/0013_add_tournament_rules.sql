-- +goose up
CREATE TYPE tournament_ruleset_enum AS ENUM (
    'KNOCKOUT',
    'ROUND_ROBIN',
    'SWISS'
);

ALTER TABLE tournaments ADD COLUMN ruleset tournament_ruleset_enum NOT NULL DEFAULT 'KNOCKOUT';
ALTER TABLE tournaments ADD COLUMN winner_id BIGINT REFERENCES users(id);

CREATE INDEX idx_tournament_winner_id ON tournaments (winner_id);

-- +goose down
DROP INDEX IF EXISTS idx_tournament_winner_id;
ALTER TABLE tournaments DROP COLUMN IF EXISTS ruleset;
ALTER TABLE tournaments DROP COLUMN IF EXISTS winner_id;
ALTER TABLE tournament_matches DROP COLUMN IF EXISTS white_id;
ALTER TABLE tournament_matches DROP COLUMN IF EXISTS black_id;
DROP TYPE IF EXISTS tournament_ruleset_enum;