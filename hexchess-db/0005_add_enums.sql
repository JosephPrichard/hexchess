-- +goose up
CREATE TYPE cause_enum AS ENUM (
    'CHECKMATE',
    'FORFEIT',
    'STALEMATE'
);

CREATE TYPE result_enum AS ENUM (
    'WHITE_WINS',
    'BLACK_WINS',
    'DRAW',
    'RANDOM'
);

CREATE TYPE color_enum AS ENUM (
    'WHITE',
    'BLACK',
    'RANDOM'
);

ALTER TABLE challenges
DROP CONSTRAINT start_color_check;

ALTER TABLE challenges
ALTER COLUMN start_color TYPE color_enum USING start_color::text::color_enum;


ALTER TABLE replays
DROP CONSTRAINT result_check;

ALTER TABLE replays
ALTER COLUMN result TYPE result_enum USING result::text::result_enum;


ALTER TABLE replays
DROP CONSTRAINT cause_check_1;

ALTER TABLE replays
ALTER COLUMN cause TYPE cause_enum USING cause::text::cause_enum;


-- +goose Down

-- Revert replays.cause to TEXT and restore constraint
ALTER TABLE replays
    ALTER COLUMN cause TYPE TEXT USING cause::text;

ALTER TABLE replays
    ADD CONSTRAINT cause_check_1
        CHECK (cause IN ('CHECKMATE', 'FORFEIT', 'STALEMATE'));

-- Revert replays.result to TEXT and restore constraint
ALTER TABLE replays
    ALTER COLUMN result TYPE TEXT USING result::text;

ALTER TABLE replays
    ADD CONSTRAINT result_check
        CHECK (result IN ('WHITE_WINS', 'BLACK_WINS', 'DRAW'));

-- Revert challenges.start_color to TEXT and restore constraint
ALTER TABLE challenges
    ALTER COLUMN start_color TYPE TEXT USING start_color::text;

ALTER TABLE challenges
    ADD CONSTRAINT start_color_check
        CHECK (start_color IN ('WHITE', 'BLACK', 'RANDOM'));

-- Drop enums after all dependencies are removed
DROP TYPE IF EXISTS cause_enum;
DROP TYPE IF EXISTS result_enum;
DROP TYPE IF EXISTS color_enum;