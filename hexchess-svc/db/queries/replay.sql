-- name: InsertReplay :one
INSERT INTO replays (game_id, white_id, black_id, result, cause, win_elo_diff, lose_elo_diff, white_elo, black_elo, played_on, mode)
VALUES (
        sqlc.arg('gameID'),
        sqlc.arg('whiteID'),
        sqlc.arg('blackID'),
        sqlc.arg('result'),
        sqlc.arg('cause'),
        sqlc.arg('winElo'),
        sqlc.arg('loseElo'),
        sqlc.arg('whiteElo'),
        sqlc.arg('blackElo'),
        COALESCE(sqlc.narg('playedOn'), CURRENT_TIMESTAMP)::TIMESTAMPTZ,
        sqlc.arg('mode'))
RETURNING id;

-- name: InsertReplayMoveHistories :exec
INSERT INTO replay_move_histories (replay_id, data)
VALUES(sqlc.arg('replayID'), sqlc.arg('data'));

-- name: SelectReplayMoveHistories :one
SELECT * FROM replay_move_histories WHERE replay_id = sqlc.arg('replayID') ORDER BY replay_id DESC LIMIT 1;

-- name: SelectReplayRowByID :one
SELECT *
FROM replays r
WHERE r.id = sqlc.arg('id');

-- name: SelectReplayByID :one
SELECT
    r.id,
    r.white_id,
    r.black_id,
    r.result,
    r.cause,
    r.played_on,
    r.win_elo_diff,
    r.lose_elo_diff,
    r.mode,
    u1.username AS white_name,
    u1.country AS white_country,
    e1.elo AS white_elo,
    u2.username AS black_name,
    u2.country AS black_country,
    e2.elo AS black_elo
FROM replays r
         -- ensures we get the white/black elo for mode at the time of retrieval
         LEFT JOIN users u1 ON u1.id = r.white_id
         LEFT JOIN users u2 ON u2.id = r.black_id
         LEFT JOIN user_mode_elos e1 ON e1.user_id = r.white_id AND e1.mode = r.mode
         LEFT JOIN user_mode_elos e2 ON e2.user_id = r.black_id AND e2.mode = r.mode
WHERE r.id = sqlc.arg('id');

-- name: SelectReplayElos :many
SELECT id, mode, played_on, white_id, black_id, white_elo, black_elo -- gets the white/black elo at the time of insertion
FROM replays
WHERE 
    (white_id = sqlc.arg('id') OR black_id = sqlc.arg('id'))
  AND
    (played_on > sqlc.narg('played_after') OR sqlc.narg('played_after') IS NULL)
ORDER BY played_on ASC;

-- name: SelectUserReplays :many
SELECT
    r.id,
    r.white_id,
    r.black_id,
    r.result,
    r.cause,
    r.played_on,
    r.win_elo_diff,
    r.lose_elo_diff,
    r.mode,
    u1.username AS white_name,
    u1.country AS white_country,
    e1.elo AS white_elo,
    u2.username AS black_name,
    u2.country AS black_country,
    e2.elo AS black_elo
FROM replays r
        -- ensures we get the white/black elo for mode at the time of retrieval
        LEFT JOIN users u1 ON u1.id = r.white_id
        LEFT JOIN users u2 ON u2.id = r.black_id
        LEFT JOIN user_mode_elos e1 ON e1.user_id = r.white_id AND e1.mode = r.mode
        LEFT JOIN user_mode_elos e2 ON e2.user_id = r.black_id AND e2.mode = r.mode
WHERE
    r.id < sqlc.arg('afterID')
  AND (
      r.white_id = sqlc.arg('userID') OR r.black_id = sqlc.arg('userID')
  )
ORDER BY r.id DESC
LIMIT sqlc.arg('perPage');

-- name: SelectReplaysExistsByIDs :many
SELECT id FROM replays WHERE id = ANY (sqlc.arg('ids')::bigint[]);

-- name: SelectReplayIDByGameID :one
SELECT id FROM replays WHERE game_id = sqlc.arg('gameID');