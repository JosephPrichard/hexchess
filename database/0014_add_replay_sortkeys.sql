-- +goose up
ALTER TABLE replays ADD COLUMN turn_count INT NOT NULL DEFAULT 0;

ALTER TABLE replays
    ADD COLUMN rating DOUBLE PRECISION
    GENERATED ALWAYS AS ((white_elo + black_elo) / 2) STORED;

ALTER TABLE replays
    ADD COLUMN played_on_as_days INT
        GENERATED ALWAYS AS (
            (EXTRACT(
                EPOCH FROM (played_on AT TIME ZONE 'UTC')::timestamp - TIMESTAMP '1970-01-01') / 86400
            )::INT
        ) STORED;

CREATE INDEX IF NOT EXISTS idx_sort_rating ON replays(rating);
CREATE INDEX IF NOT EXISTS idx_sort_turncount ON replays(turn_count);

CREATE INDEX IF NOT EXISTS idx_blackid_sort_id ON replays(black_id, id);
CREATE INDEX IF NOT EXISTS idx_blackid_sort_rating ON replays(black_id, rating);
CREATE INDEX IF NOT EXISTS idx_blackid_sort_turncount ON replays(black_id, turn_count);

CREATE INDEX IF NOT EXISTS idx_whiteid_sort_id ON replays(white_id, id);
CREATE INDEX IF NOT EXISTS idx_whiteid_sort_rating ON replays(white_id, rating);
CREATE INDEX IF NOT EXISTS idx_whiteid_sort_turncount ON replays(white_id, turn_count);

CREATE INDEX IF NOT EXISTS idx_playedon ON replays(played_on_as_days);

-- +goose down
ALTER TABLE replays DROP COLUMN turn_count;
ALTER TABLE replays DROP COLUMN rating;
ALTER TABLE replays DROP COLUMN played_on_as_days;