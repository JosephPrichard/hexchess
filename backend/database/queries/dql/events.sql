-- name: SelectByEventKeyID :one
SELECT data
FROM event_keys
WHERE id = sqlc.arg('arg');