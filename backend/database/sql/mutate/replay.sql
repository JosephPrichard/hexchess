-- name: InsertReplay :one
INSERT INTO replays (game_id,
                     white_id,
                     black_id,
                     result,
                     cause,
                     win_elo_diff,
                     lose_elo_diff,
                     white_elo,
                     black_elo,
                     played_on,
                     mode,
                     turn_count)
VALUES (sqlc.arg('gameID'),
        sqlc.arg('whiteID'),
        sqlc.arg('blackID'),
        sqlc.arg('result'),
        sqlc.arg('cause'),
        sqlc.arg('winElo'),
        sqlc.arg('loseElo'),
        sqlc.arg('whiteElo'),
        sqlc.arg('blackElo'),
        COALESCE(sqlc.narg('playedOn'), CURRENT_TIMESTAMP)::TIMESTAMPTZ,
        sqlc.arg('mode'),
        sqlc.arg('turnCount')) RETURNING id;

-- name: UpsertReplayMoveHistories :exec
INSERT INTO replay_move_histories (replay_id, data)
VALUES (sqlc.arg('replayID'), sqlc.arg('data')) ON CONFLICT
ON CONSTRAINT replay_move_histories_pkey
    DO
UPDATE
    SET data = sqlc.arg('data');