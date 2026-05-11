-- +goose up
CREATE TABLE replay_move_histories (
    replay_id BIGINT PRIMARY KEY,
    data      BYTEA NOT NULL,

    CONSTRAINT fk_replay_id
    FOREIGN KEY (replay_id)
    REFERENCES replays(id)
    ON DELETE CASCADE
);

-- +goose down
DROP TABLE IF EXISTS replay_move_histories;