-- name: InsertUser :one
INSERT INTO users (username,
                   country,
                   password,
                   salt,
                   google_account_id,
                   joined_on)
VALUES (sqlc.arg('username'),
        sqlc.arg('country'),
        sqlc.arg('password'),
        sqlc.arg('salt'),
        sqlc.arg('googleAccountID'),
        COALESCE(sqlc.narg('joined_on')::TIMESTAMPTZ, CURRENT_TIMESTAMP)) RETURNING
    id,
    username,
    country,
    bio,
    joined_on;

-- name: BatchInsertUser :batchone
INSERT INTO users (username,
                   country,
                   password,
                   salt,
                   joined_on)
VALUES (sqlc.arg('username'),
        sqlc.arg('country'),
        sqlc.arg('password'),
        sqlc.arg('salt'),
        sqlc.arg('joinedOn')) RETURNING
    id,
    username,
    country,
    bio,
    joined_on;

-- name: UpdateUser :one
UPDATE users
SET username = COALESCE(sqlc.narg('username'), username),
    country  = COALESCE(sqlc.narg('country'), country),
    bio      = COALESCE(sqlc.narg('bio'), bio)
WHERE id = sqlc.arg('id') RETURNING
    id,
    username,
    country,
    bio,
    joined_on;

-- name: IncrLoginAttempts :exec
UPDATE users
SET last_login_attempt = CURRENT_TIMESTAMP,
    login_attempts     = login_attempts + 1
WHERE id = sqlc.arg('id');

-- name: ResetLoginAttempts :exec
UPDATE users
SET last_login_attempt = CURRENT_TIMESTAMP,
    login_attempts     = 0
WHERE id = sqlc.arg('id');

-- name: UpdatePassword :exec
UPDATE users
SET password = sqlc.arg('password'),
    salt     = sqlc.arg('salt')
WHERE id = sqlc.arg('id');

-- name: UpsertUserElo :batchexec
INSERT INTO user_mode_elos AS u (user_id, mode, elo, highest_elo, wins, losses, draws)
VALUES (
    sqlc.arg('userID'), sqlc.arg('mode'), COALESCE (sqlc.narg('elo')::FLOAT8, sqlc.arg('defaultElo')::FLOAT8), GREATEST(sqlc.narg('elo'), sqlc.arg('defaultElo')), sqlc.arg('wins'), sqlc.arg('losses'), sqlc.arg('draws'))
ON CONFLICT ON CONSTRAINT user_mode_elos_pkey
    DO
UPDATE SET
    elo = COALESCE (sqlc.narg('elo'), u.elo),
    highest_elo = GREATEST(u.highest_elo, sqlc.narg('elo')),
    wins = u.wins + sqlc.arg('wins'),
    losses = u.losses + sqlc.arg('losses'),
    draws = u.draws + sqlc.arg('draws');