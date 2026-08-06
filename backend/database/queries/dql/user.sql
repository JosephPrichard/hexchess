-- name: SelectLoginByName :one
SELECT id, username, country, password, salt, login_attempts, last_login_attempt
FROM users
WHERE UPPER(username) = UPPER(sqlc.arg('username')::TEXT);

-- name: SelectByGoogleAccountID :one
SELECT id, username, country
FROM users
WHERE google_account_id = sqlc.arg('googleAccountID');

-- name: SelectUserByID :one
SELECT id,
       username,
       country,
       bio,
       joined_on
FROM users
WHERE id = sqlc.arg('id');

-- name: SelectUserWithEloByIDs :many
SELECT u.id,
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
WHERE id = ANY (sqlc.arg('ids')::bigint[]);

-- name: SelectUserWithEloByID :one
SELECT u.id,
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
WHERE id = sqlc.arg('id');

-- name: SelectExistsUsersByIDs :many
SELECT id
FROM users
WHERE id = ANY (sqlc.arg('ids')::bigint[]);

-- name: SelectUsersBySimilarity :many
SELECT id,
       username,
       country,
       (username <-> sqlc.arg('username')) ::BIGINT AS rank
FROM users
WHERE username % sqlc.arg('username')::TEXT
ORDER BY rank DESC
    LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: SelectUserModeElosByIDs :many
SELECT user_id, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = ANY (sqlc.arg('id')::bigint[])
  AND mode = sqlc.arg('mode');

-- name: SelectUserElosByID :many
SELECT user_id, mode, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = sqlc.arg('id');

-- name: SelectUserElosByIDs :many
SELECT user_id, mode, elo, highest_elo, wins, losses, draws
FROM user_mode_elos
WHERE user_id = ANY (sqlc.arg('id')::bigint[]);

-- name: SelectUsersByIDs :many
SELECT id, username, country
FROM users
WHERE id = ANY (sqlc.arg('id')::bigint[]);

-- name: SelectUserIDsByNames :many
SELECT id, username
FROM users
WHERE username = ANY (sqlc.arg('usernames')::text[]);

-- name: SelectEloList :many
SELECT user_id, elo
FROM user_mode_elos
WHERE user_id > sqlc.arg('id')
  AND mode = sqlc.arg('mode')
ORDER BY user_id LIMIT sqlc.arg('limit');