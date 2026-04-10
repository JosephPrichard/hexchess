-- name: InsertOutboxQueue :exec
INSERT INTO outbox_queue (type, data, scheduled_on)
VALUES (sqlc.arg('type'), sqlc.arg('data'), sqlc.narg('scheduled_on'));

-- name: SelectOutboxQueue :many
SELECT id, type, data
FROM outbox_queue
WHERE processed_on IS NULL AND type = sqlc.arg('type') AND (scheduled_on IS NULL OR scheduled_on < sqlc.arg('scheduled_on'))
ORDER BY id
LIMIT sqlc.arg('limit')
FOR UPDATE SKIP LOCKED;

-- name: UpdateOutboxQueueProcessedByID :exec
UPDATE outbox_queue
SET processed_on = sqlc.arg('processed_time')
WHERE id = ANY (sqlc.arg('ids')::bigint[]);