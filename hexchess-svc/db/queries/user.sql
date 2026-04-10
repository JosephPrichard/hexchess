-- name: InsertUser :one
INSERT INTO users (
       username,
       country,
       password,
       salt,
       google_account_id,
       joined_on)
VALUES (
        sqlc.arg('username'),
        sqlc.arg('country'),
        sqlc.arg('password'),
        sqlc.arg('salt'),
        sqlc.arg('google_account_id'),
        COALESCE(sqlc.narg('joined_on')::TIMESTAMPTZ, CURRENT_TIMESTAMP))
RETURNING
    id,
    username,
    country,
    bio,
    joined_on;

-- name: BatchInsertUser :batchone
INSERT INTO users (
       username,
       country,
       password,
       salt,
       joined_on)
VALUES (
        sqlc.arg('username'),
        sqlc.arg('country'),
        sqlc.arg('password'),
        sqlc.arg('salt'),
        sqlc.arg('joined_on'))
RETURNING
    id,
    username,
    country,
    bio,
    joined_on;

-- name: SelectLoginByName :one
SELECT id, username, country, password, salt, login_attempts, last_login_attempt
FROM users
WHERE UPPER(username) = UPPER(sqlc.arg('username')::TEXT);

-- name: SelectByGoogleAccountID :one
SELECT id, username, country
FROM users
WHERE google_account_id = sqlc.arg('googleAccountID');

-- name: SelectUserByID :one
SELECT
    id,
    username,
    country,
    bio,
    joined_on
FROM users
WHERE id = sqlc.arg('id');

-- name: SelectUserWithEloByIDs :many
SELECT
    u.id,
    u.username,
    u.country,
    u.bio,
    u.joined_on,
    e.elo,
    e.highest_elo,
    e.wins,
    e.losses,
    e.draws
FROM users u
    LEFT JOIN user_mode_elos e
        ON u.id = e.user_id AND e.mode = sqlc.arg('mode')
WHERE id = ANY(sqlc.arg('ids')::bigint[]);

-- name: SelectExistsUsersByIDs :many
SELECT id FROM users WHERE id = ANY (sqlc.arg('ids')::bigint[]);

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
    bio,
    joined_on;

-- name: SelectUsersBySimilarity :many
SELECT
    id,
    username,
    country,
    (username <-> sqlc.arg('username'))::BIGINT AS rank
FROM users
WHERE username % sqlc.arg('username')::TEXT
ORDER BY rank DESC
    LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

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

-- name: SelectUserModeElosByIDs :many
SELECT user_id, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = ANY(sqlc.arg('id')::bigint[]) AND mode = sqlc.arg('mode');

-- name: SelectUserElosByID :many
SELECT user_id, mode, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = sqlc.arg('id');

-- name: SelectUserElosByIDs :many
SELECT user_id, mode, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = ANY(sqlc.arg('id')::bigint[]);

-- name: SelectUserPlayerDataByIDs :many
SELECT id, username, country
FROM users
WHERE id = ANY(sqlc.arg('id')::bigint[]);

-- name: UpsertUserElo :batchexec
INSERT INTO user_mode_elos AS u (user_id, mode, elo, highest_elo, wins, losses, draws)
VALUES (
    sqlc.arg('userID'),
    sqlc.arg('mode'),
    COALESCE(sqlc.narg('elo')::FLOAT8, sqlc.arg('defaultElo')::FLOAT8),
    GREATEST(sqlc.narg('elo'), sqlc.arg('defaultElo')),
    sqlc.arg('wins'),
    sqlc.arg('losses'),
    sqlc.arg('draws'))
ON CONFLICT ON CONSTRAINT user_mode_elos_pkey
DO UPDATE
SET
    elo = COALESCE(sqlc.narg('elo'), u.elo),
    highest_elo = GREATEST(u.highest_elo, sqlc.narg('elo')),
    wins = u.wins + sqlc.arg('wins'),
    losses = u.losses + sqlc.arg('losses'),
    draws = u.draws + sqlc.arg('draws');

-- name: SelectEloList :many
SELECT user_id, elo FROM user_mode_elos
WHERE user_id > sqlc.arg('id') AND mode = sqlc.arg('mode')
ORDER BY user_id
LIMIT sqlc.arg('limit');