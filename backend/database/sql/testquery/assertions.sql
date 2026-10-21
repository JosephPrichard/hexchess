-- name: SelectTournamentByID :one
SELECT *
FROM tournaments
WHERE tournament_key = sqlc.arg('tournamentKey')::uuid;

-- name: SelectParticipants :many
SELECT *
FROM tournament_participants
WHERE tournament_key = sqlc.arg('tournamentKey')::uuid;

-- name: SelectMatches :many
SELECT *
FROM tournament_matches
WHERE tournament_key = sqlc.arg('tournamentKey')::uuid;

-- name: SelectUserModeElosByIDs :many
SELECT *
FROM user_mode_elos
WHERE user_id = ANY (sqlc.arg('id')::bigint[])
  AND mode = sqlc.arg('mode');

-- name: SelectByEventKeyID :one
SELECT data
FROM event_keys
WHERE id = sqlc.arg('arg');

-- name: SelectGameMeta :one
SELECT *
FROM games_metadata
WHERE game_id = sqlc.arg('gameID');

-- name: SelectChallenge :one
SELECT *
FROM challenges
WHERE challenger_id = sqlc.arg('challengerID')
  AND challengee_id = sqlc.arg('challengeeID');

-- name: SelectReplayByID :one
SELECT *
FROM replays r
WHERE r.id = sqlc.arg('id');