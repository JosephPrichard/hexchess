-- name: UpdateGameMeta :one
INSERT INTO games_metadata (game_id, mode, white_id, black_id, updated_on, ordering)
VALUES (sqlc.arg('gameID'), sqlc.arg('mode'), sqlc.narg('whiteID'), sqlc.narg('blackID'), sqlc.arg('updatedOn'), nextval('games_metadata_ordering_seq'))
ON CONFLICT (game_id)
DO UPDATE SET
    game_id = sqlc.arg('gameID'),
    mode = sqlc.arg('mode'),
    white_id = sqlc.narg('whiteID'),
    black_id = sqlc.narg('blackID'),
    updated_on = sqlc.arg('updatedOn'),
    ordering = nextval('games_metadata_ordering_seq')
RETURNING (xmax = 0) AS is_new_row;

-- name: SelectGameMetasCount :one
SELECT total FROM games_metadata_count;

-- name: SelectGameMetas :many
SELECT
    gm.ordering,
    gm.game_id,
    gm.mode,
    gm.white_id,
    gm.black_id,
    u1.username AS white_name,
    u1.country AS white_country,
    u2.username AS black_name,
    u2.country AS black_country
FROM games_metadata gm
LEFT JOIN users u1 ON u1.id = white_id
LEFT JOIN users u2 ON u2.id = black_id
WHERE
    (sqlc.narg('participantID')::BIGINT IS NULL OR
     white_id = sqlc.narg('participantID') OR
     black_id = sqlc.narg('participantID')) AND
    gm.ordering < sqlc.arg('afterOrdering')
ORDER BY ordering DESC
LIMIT sqlc.narg('perPage')::INT;