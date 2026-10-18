-- name: InsertEventKey :exec
INSERT INTO event_keys (id, data)
VALUES (sqlc.arg('id'), sqlc.arg('data'));