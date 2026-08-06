-- name: SelectGameMetasCount :one
SELECT total
FROM games_metadata_count;

-- name: SelectGameMetas :many
SELECT gm.ordering,
       gm.game_id,
       gm.mode,
       gm.white_id,
       gm.black_id,
       u1.username AS white_name,
       u1.country  AS white_country,
       u2.username AS black_name,
       u2.country  AS black_country
FROM games_metadata gm
         LEFT JOIN users u1 ON u1.id = white_id
         LEFT JOIN users u2 ON u2.id = black_id
WHERE (sqlc.narg('participantID')::BIGINT IS NULL OR
     white_id = sqlc.narg('participantID') OR
     black_id = sqlc.narg('participantID'))
  AND gm.ordering < sqlc.arg('afterOrdering')
ORDER BY ordering DESC LIMIT sqlc.narg('perPage')::INT;

-- name: SelectGameMeta :one
SELECT *
FROM games_metadata
WHERE game_id = sqlc.arg('gameID');