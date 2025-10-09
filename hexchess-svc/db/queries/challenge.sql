-- ======================
-- Challenges Queries
-- ======================

-- name: InsertChallenge :one
WITH inserted_challenges AS (
    INSERT INTO challenges (challenger_id, challengee_id, time_control, start_color, made_on)
    VALUES (sqlc.arg('challengerID'), sqlc.arg('challengeeID'), sqlc.arg('timeControl'), sqlc.arg('startColor'), sqlc.arg('madeOn'))
    RETURNING *
)
SELECT
    c.challenger_id,
    c.challengee_id,
    u2.username AS challenger_name,
    u2.country AS challenger_country,
    u2.elo AS challenger_elo,
    u1.username AS challengee_name,
    u1.country AS challengee_country,
    u1.elo AS challengee_elo,
    c.time_control,
    c.start_color,
    c.made_on
FROM inserted_challenges c
         INNER JOIN users u1 ON u1.id = c.challengee_id
         INNER JOIN users u2 ON u2.id = c.challenger_id;

-- name: DeleteChallenge :one
DELETE FROM challenges
WHERE challenger_id = sqlc.arg('challengerID') AND challengee_id = sqlc.arg('challengeeID')
    RETURNING challenger_id, challengee_id, time_control, start_color;

-- name: SelectChallengesByParticipant :many
SELECT
    c.challenger_id,
    c.challengee_id,
    u2.username AS challenger_name,
    u2.country AS challenger_country,
    u2.elo AS challenger_elo,
    u1.username AS challengee_name,
    u1.country AS challengee_country,
    u1.elo AS challengee_elo,
    c.time_control,
    c.start_color,
    c.made_on
FROM challenges c
         INNER JOIN users u1 ON u1.id = c.challengee_id
         INNER JOIN users u2 ON u2.id = c.challenger_id
WHERE (sqlc.narg('challengerID')::BIGINT IS NULL OR challenger_id = sqlc.narg('challengerID')::BIGINT)
  AND (sqlc.narg('challengeeID')::BIGINT IS NULL OR challengee_id = sqlc.narg('challengeeID')::BIGINT)
  AND made_on >= sqlc.arg('since')
ORDER BY made_on DESC;

-- name: DeleteExpiredChallenges :exec
DELETE FROM challenges
WHERE (challengee_id = sqlc.arg('userID') OR challenger_id = sqlc.arg('userID'))
  AND made_on < sqlc.arg('expireTime');
