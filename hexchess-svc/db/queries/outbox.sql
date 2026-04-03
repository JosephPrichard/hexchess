-- name: InsertOutboxQueue :exec
INSERT INTO outbox_queue (type, data)
VALUES (sqlc.arg('type'), sqlc.arg('data'));

-- name: SelectOutboxQueue :many
SELECT type, data
FROM outbox_queue
WHERE processed_on IS NULL
ORDER BY id
LIMIT sqlc.arg('limit')
FOR UPDATE SKIP LOCKED;