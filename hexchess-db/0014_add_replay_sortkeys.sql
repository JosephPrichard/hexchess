-- +goose up
ALTER TABLE replays ADD COLUMN turn_count INT NOT NULL DEFAULT 0;
ALTER TABLE replays ADD COLUMN rating DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE replays ALTER COLUMN turn_count DROP DEFAULT;
ALTER TABLE replays ALTER COLUMN rating DROP DEFAULT;

DROP INDEX idx_black_id;
DROP INDEX idx_both_ids;
DROP INDEX idx_both_ids_played_on;
DROP INDEX idx_white_id;

CREATE INDEX idx_sort_rating ON replays(rating);
CREATE INDEX idx_sort_turncount ON replays(turn_count);

CREATE INDEX idx_blackid_sort_id ON replays(black_id, id);
CREATE INDEX idx_blackid_sort_rating ON replays(black_id, rating);
CREATE INDEX idx_blackid_sort_turncount ON replays(black_id, turn_count);

CREATE INDEX idx_whiteid_sort_id ON replays(white_id, id);
CREATE INDEX idx_whiteid_sort_rating ON replays(white_id, rating);
CREATE INDEX idx_whiteid_sort_turncount ON replays(white_id, turn_count);

CREATE INDEX idx_playedon ON replays(played_on);

-- +goose down
ALTER TABLE replays DROP COLUMN turn_count;
ALTER TABLE replays DROP COLUMN rating;

CREATE INDEX idx_black_id ON replays(black_id, id);
CREATE INDEX idx_both_ids ON replays(white_id, black_id, id);
CREATE INDEX idx_both_ids_played_on ON replays(white_id, black_id, id);
CREATE INDEX idx_white_id ON replays(white_id, id);

DROP INDEX idx_sort_rating;
DROP INDEX idx_sort_turncount;

DROP INDEX idx_blackid_sort_id;
DROP INDEX idx_blackid_sort_rating;
DROP INDEX idx_blackid_sort_turncount;

DROP INDEX idx_whiteid_sort_id;
DROP INDEX idx_whiteid_sort_rating;
DROP INDEX idx_whiteid_sort_turncount;

DROP INDEX idx_playedon;