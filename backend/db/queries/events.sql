-- name: InsertEventKey :exec
INSERT INTO event_keys (id, data)
VALUES (sqlc.arg('id'), sqlc.arg('data'));

-- name: SelectByEventKeyID :one
SELECT data
FROM event_keys
WHERE id = sqlc.arg('arg');