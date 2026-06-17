-- name: InsertEvent :exec
INSERT INTO event_keys (id, data)
VALUES (sqlc.arg('id'), sqlc.arg('data'));

-- name: SelectByEventID :one
SELECT data
FROM event_keys
WHERE id = sqlc.arg('arg');