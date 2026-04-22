-- name: InsertOutboxQueue :exec
INSERT INTO outbox_queue (type, data, created_on, scheduled_on)
VALUES (sqlc.arg('type'), sqlc.arg('data'), COALESCE(sqlc.narg('created_on')::timestamptz, CURRENT_TIMESTAMP), sqlc.narg('scheduled_on'));

-- name: SelectOutboxQueueByPolling :many
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

-- name: SelectALLOutboxQueue :many
SELECT * FROM outbox_queue ORDER BY id; -- intended for test asserts; use at your own risk.