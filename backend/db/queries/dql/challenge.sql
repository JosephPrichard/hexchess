-- name: CountReceivedChallenges :one
SELECT COUNT(*)
FROM challenges
WHERE challengee_id = sqlc.arg('userID');

-- name: SelectChallenge :one
SELECT *
FROM challenges
WHERE challenger_id = sqlc.arg('challengerID')
  AND challengee_id = sqlc.arg('challengeeID');

-- name: SelectChallengesByParticipant :many
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
FROM challenges c
         INNER JOIN users u1
                    ON u1.id = c.challengee_id
         INNER JOIN users u2
                    ON u2.id = c.challenger_id
         LEFT JOIN user_mode_elos e1
                   ON e1.user_id = c.challengee_id AND e1.mode = c.mode
         LEFT JOIN user_mode_elos e2
                   ON e2.user_id = c.challenger_id AND e2.mode = c.mode
WHERE (
    sqlc.narg('challengerID')::BIGINT IS NULL
        OR
        challenger_id = sqlc.narg('challengerID')::BIGINT
    )
  AND (
    sqlc.narg('challengeeID')::BIGINT IS NULL
        OR
        challengee_id = sqlc.narg('challengeeID')::BIGINT
    )
  AND made_on >= sqlc.arg('since')
ORDER BY made_on DESC;