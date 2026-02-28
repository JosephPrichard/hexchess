-- +goose up
ALTER TABLE replays
DROP CONSTRAINT IF EXISTS cause_check;

ALTER TABLE replays
    ADD CONSTRAINT cause_check_1
    CHECK (cause IN ('CHECKMATE', 'FORFEIT', 'STALEMATE'));

-- +goose Down
ALTER TABLE replays
DROP CONSTRAINT IF EXISTS cause_check_1;

ALTER TABLE replays
    ADD CONSTRAINT cause_check
        CHECK (cause IN ('CHECKMATE', 'FORFEIT'));