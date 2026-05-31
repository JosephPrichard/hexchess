-- name: InsertEvent :exec
INSERT INTO events (id, data)
VALUES (sqlc.arg('id'), sqlc.arg('data'));

-- name: SelectByEventID :one
SELECT data FROM events WHERE id = sqlc.arg('arg');