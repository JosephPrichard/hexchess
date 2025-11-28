-- name: InsertUser :one
INSERT INTO users (username, country, elo, highest_elo, wins, losses, password, salt, google_account_id)
VALUES (sqlc.arg('username'), sqlc.arg('country'), sqlc.arg('elo'), sqlc.arg('highestElo'), sqlc.arg('wins'), sqlc.arg('losses'), sqlc.arg('password'), sqlc.arg('salt'), sqlc.arg('google_account_id'))
RETURNING id, username, country, elo, highest_elo, wins, losses, bio, joined_on;

-- name: BatchInsertUser :batchone
INSERT INTO users (username, country, elo, highest_elo, wins, losses, password, salt, google_account_id)
VALUES (sqlc.arg('username'), sqlc.arg('country'), sqlc.arg('elo'), sqlc.arg('highestElo'), sqlc.arg('wins'), sqlc.arg('losses'), sqlc.arg('password'), sqlc.arg('salt'), sqlc.arg('google_account_id'))
RETURNING id, username, country, elo, highest_elo, wins, losses, bio, joined_on;

-- name: SelectLoginByName :one
SELECT id, username, country, elo, password, salt, login_attempts, last_login_attempt
FROM users
WHERE UPPER(username) = UPPER(sqlc.arg('username')::TEXT);

-- name: SelectByGoogleAccountID :one
SELECT id, username, country, elo
FROM users
WHERE google_account_id = sqlc.arg('googleAccountID');

-- name: SelectUserByID :one
SELECT
    id,
    username,
    country,
    elo,
    highest_elo,
    wins,
    losses,
    bio,
    joined_on
FROM users
WHERE id = sqlc.arg('id');

-- name: SelectUsersByIDs :many
SELECT
    id,
    username,
    country,
    elo,
    highest_elo,
    wins,
    losses,
    bio,
    joined_on
FROM users
WHERE id = ANY(sqlc.arg('ids')::bigint[]);

-- name: SelectAllUsers :many
SELECT
    id,
    username,
    country,
    elo,
    wins,
    losses
FROM users;

-- name: SelectEloListAfterID :many
SELECT
    id,
    elo
FROM users
WHERE id > sqlc.arg('afterID')
ORDER BY id
    LIMIT sqlc.arg('limit');

-- name: UpdateUser :one
UPDATE users
SET
    username = COALESCE(sqlc.narg('username'), username),
    country = COALESCE(sqlc.narg('country'), country),
    bio = COALESCE(sqlc.narg('bio'), bio)
WHERE id = sqlc.arg('id')
RETURNING
    id,
    username,
    country,
    elo,
    highest_elo,
    wins,
    losses,
    bio,
    joined_on;

-- name: SelectUsersBySimilarity :many
SELECT
    id,
    username,
    country,
    elo,
    wins,
    losses,
    (username <-> sqlc.arg('username'))::BIGINT AS rank
FROM users
WHERE username % sqlc.arg('username')::TEXT
ORDER BY rank DESC
    LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: GetEloList :many
SELECT id, elo FROM users WHERE id > sqlc.arg('id') ORDER BY id LIMIT sqlc.arg('limit');

-- name: IncrLoginAttempts :exec
UPDATE users
SET last_login_attempt = CURRENT_TIMESTAMP,
    login_attempts = login_attempts + 1
WHERE id = sqlc.arg('id');

-- name: ResetLoginAttempts :exec
UPDATE users
SET last_login_attempt = CURRENT_TIMESTAMP,
    login_attempts = 0
WHERE id = sqlc.arg('id');

-- name: UpdatePassword :exec
UPDATE users
SET password = sqlc.arg('password'), salt = sqlc.arg('salt')
WHERE id = sqlc.arg('id');

-- name: GetElo :one
SELECT elo
FROM users
WHERE id = sqlc.arg('id');

-- name: UpdateWins :exec
UPDATE users
SET elo = sqlc.arg('elo'),
    wins = wins + 1,
    highest_elo = GREATEST(highest_elo, sqlc.arg('elo'))
WHERE id = sqlc.arg('id');

-- name: UpdateLosses :exec
UPDATE users
SET elo = sqlc.arg('elo'),
    losses = losses + 1
WHERE id = sqlc.arg('id');