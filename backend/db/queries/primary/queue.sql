-- name: InsertQueue :exec
INSERT INTO event_queue (type, data, created_on, scheduled_on)
VALUES (sqlc.arg('type'), sqlc.arg('data'), COALESCE(sqlc.narg('createdOn')::timestamptz, CURRENT_TIMESTAMP),
        sqlc.narg('scheduledOn'));

-- name: SelectQueueByPolling :many
SELECT id, type, data
FROM event_queue
WHERE processed_on IS NULL
  AND type = sqlc.arg('type')
  AND (scheduled_on IS NULL OR scheduled_on < sqlc.arg('scheduledOn'))
ORDER BY id LIMIT sqlc.arg('limit')
FOR
UPDATE SKIP LOCKED;

-- name: UpdateQueueProcessedByID :exec
UPDATE event_queue
SET processed_on = sqlc.arg('processedTime')
WHERE id = ANY (sqlc.arg('ids')::bigint[]);

-- name: SelectALLQueue :many
SELECT *
FROM event_queue
ORDER BY id; -- intended for test asserts; use at your own risk.