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
