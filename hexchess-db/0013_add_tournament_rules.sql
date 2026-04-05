-- +goose up
CREATE TYPE tournament_ruleset_enum AS ENUM (
    'KNOCKOUT',
    'ROUND_ROBIN',
    'SWISS'
);

ALTER TABLE tournaments ADD COLUMN ruleset tournament_ruleset_enum NOT NULL DEFAULT 'KNOCKOUT';

-- +goose down
ALTER TABLE tournaments DROP COLUMN IF EXISTS ruleset;
DROP TYPE IF EXISTS tournament_ruleset_enum;