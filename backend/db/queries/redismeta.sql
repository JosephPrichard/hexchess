-- name: InsertRedisEventMetas :batchexec
INSERT INTO redis_queue_metadata (group_id, stream_name, consumed_on, processed_on) 
VALUES (sqlc.arg('groupId'), sqlc.arg('streamName'), sqlc.arg('consumedOn'), sqlc.arg('processedOn')) ON CONFLICT 
ON CONSTRAINT redis_queue_metadata_idx DO NOTHING;;