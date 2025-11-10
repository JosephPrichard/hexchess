-- name: InsertReplay :one
INSERT INTO replays (white_id, black_id, result, cause, win_elo, lose_elo, move_list)
VALUES (sqlc.arg('whiteID'), sqlc.arg('blackID'), sqlc.arg('result'), sqlc.arg('cause'), sqlc.arg('winElo'), sqlc.arg('loseElo'), sqlc.arg('moveList'))
RETURNING id;

-- name: GetReplayByID :one
SELECT
    r.id,
    r.white_id,
    r.black_id,
    r.result,
    r.cause,
    r.played_on,
    r.win_elo,
    r.lose_elo,
    u1.username AS white_name,
    u1.country AS white_country,
    u1.elo AS white_elo,
    u2.username AS black_name,
    u2.country AS black_country,
    u2.elo AS black_elo
FROM replays r
         INNER JOIN users u1 ON u1.id = r.white_id
         INNER JOIN users u2 ON u2.id = r.black_id
WHERE r.id = sqlc.arg('id');

-- name: GetReplayMoveHistory :one
SELECT move_list AS move_history_bytes
FROM replays
WHERE id = sqlc.arg('id');

-- name: GetUserReplays :many
SELECT
    r.id,
    r.white_id,
    r.black_id,
    r.result,
    r.cause,
    r.played_on,
    r.win_elo,
    r.lose_elo,
    u1.username AS white_name,
    u1.country AS white_country,
    u1.elo AS white_elo,
    u2.username AS black_name,
    u2.country AS black_country,
    u2.elo AS black_elo
FROM replays r
         INNER JOIN users u1 ON u1.id = r.white_id
         INNER JOIN users u2 ON u2.id = r.black_id
WHERE r.id < sqlc.arg('afterID')
  AND (r.white_id = sqlc.arg('userID') OR r.black_id = sqlc.arg('userID'))
ORDER BY r.id DESC
    LIMIT sqlc.arg('perPage');