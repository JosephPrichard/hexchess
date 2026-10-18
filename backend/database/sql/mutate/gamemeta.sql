-- name: UpdateGameMeta :one
INSERT INTO games_metadata (game_id, mode, white_id, black_id, updated_on, ordering)
VALUES (sqlc.arg('gameID'), sqlc.arg('mode'), sqlc.narg('whiteID'), sqlc.narg('blackID'), sqlc.arg('updatedOn'),
        nextval('games_metadata_ordering_seq')) ON CONFLICT (game_id)
DO
UPDATE SET
    mode = sqlc.arg('mode'),
    white_id = sqlc.narg('whiteID'),
    black_id = sqlc.narg('blackID'),
    updated_on = sqlc.arg('updatedOn'),
    ordering = nextval('games_metadata_ordering_seq')
    RETURNING (xmax = 0) AS is_new_row, (SELECT total FROM games_metadata_count) AS count;