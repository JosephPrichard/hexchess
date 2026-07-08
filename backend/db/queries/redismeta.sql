-- name: InsertRedisEventMetas :batchexec
INSERT INTO redis_queue_metrics (group_id, event_id, stream_name, consumed_on, processed_on) 
VALUES (sqlc.arg('groupId'), sqlc.arg('eventId'), sqlc.arg('streamName'), sqlc.arg('consumedOn'), sqlc.arg('processedOn')) ON CONFLICT 
ON CONSTRAINT redis_queue_metrics_pkey DO NOTHING;