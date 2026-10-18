-- name: InsertChallenge :one
WITH inserted_challenges AS (
INSERT
INTO challenges (challenger_id, challengee_id, mode, start_color, made_on)
VALUES (sqlc.arg('challengerID'), sqlc.arg('challengeeID'), sqlc.arg('mode'), sqlc.arg('startColor'), sqlc.arg('madeOn'))
    RETURNING *
    )
SELECT c.challenger_id,
       c.challengee_id,
       u2.username AS challenger_name,
       u2.country  AS challenger_country,
       e2.elo      AS challenger_elo,
       u1.username AS challengee_name,
       u1.country  AS challengee_country,
       e1.elo      AS challengee_elo,
       c.mode,
       c.start_color,
       c.made_on
FROM inserted_challenges c
         INNER JOIN users u1
                    ON u1.id = c.challengee_id
         INNER JOIN users u2
                    ON u2.id = c.challenger_id
         LEFT JOIN user_mode_elos e1
                   ON e1.user_id = c.challengee_id AND e1.mode = c.mode
         LEFT JOIN user_mode_elos e2
                   ON e2.user_id = c.challenger_id AND e2.mode = c.mode;

-- name: BatchInsertChallenge :batchexec
INSERT INTO challenges (challenger_id, challengee_id, mode, start_color, made_on)
VALUES (sqlc.arg('challengerID'), sqlc.arg('challengeeID'), sqlc.arg('mode'), sqlc.arg('startColor'),
        sqlc.arg('madeOn'));

-- name: DeleteChallenge :one
DELETE
FROM challenges
WHERE challenger_id = sqlc.arg('challengerID')
  AND challengee_id = sqlc.arg('challengeeID') RETURNING challenger_id, challengee_id, mode, start_color;

-- name: DeleteExpiredChallenges :exec
DELETE
FROM challenges
WHERE (challengee_id = sqlc.arg('userID')
    OR challenger_id = sqlc.arg('userID'))
  AND made_on < sqlc.arg('before');