-- name: InsertTournament :one
INSERT INTO tournaments (tournament_key, name, rounds, ruleset, status, countdown, countdown_started_on, created_on,
                         created_by, updated_on, mode)
VALUES (sqlc.arg('tournamentKey')::uuid,
        sqlc.arg('name'),
        sqlc.arg('rounds'),
        sqlc.arg('ruleset'),
        sqlc.arg('status'),
        sqlc.arg('countdown'),
        sqlc.arg('countdownStartedOn'),
        sqlc.arg('createdOn'),
        sqlc.arg('createdBy'),
        sqlc.arg('updatedOn'),
        sqlc.arg('mode')) RETURNING id;

-- name: InsertTournamentParticipant :exec
INSERT INTO tournament_participants (tournament_key, user_id, joined_on)
VALUES (sqlc.arg('tournamentKey')::uuid, sqlc.arg('userID'), sqlc.arg('joinedOn'));

-- name: BatchInsertTournamentParticipant :batchexec
INSERT INTO tournament_participants (tournament_key, user_id, joined_on)
VALUES (sqlc.arg('tournamentKey')::uuid, sqlc.arg('userID'), sqlc.arg('joinedOn')::timestamptz);

-- name: BatchInsertTournamentMatch :batchexec
INSERT INTO tournament_matches (tournament_key, game_id, white_id, black_id, round, created_on)
VALUES (sqlc.arg('tournamentKey')::uuid,
        sqlc.arg('gameID'),
        sqlc.arg('whiteID'),
        sqlc.arg('blackID'),
        sqlc.arg('round'),
        COALESCE(sqlc.narg('createdOn')::timestamptz, CURRENT_TIMESTAMP)) ON CONFLICT
ON CONSTRAINT tournament_matches_pkey DO NOTHING;

-- name: UpdateTournamentStatus :exec
UPDATE tournaments
SET status               = sqlc.arg('status'),
    updated_on           = COALESCE(sqlc.narg('updatedOn'), CURRENT_TIMESTAMP),
    rounds               = COALESCE(sqlc.narg('rounds'), rounds),
    winner_id            = COALESCE(sqlc.narg('winnerID'), winner_id),
    countdown_started_on = COALESCE(sqlc.narg('countdownStartedOn'), countdown_started_on)
WHERE tournament_key = sqlc.arg('tournamentKey');

-- name: DeleteTournamentParticipant :many
DELETE
FROM tournament_participants tp USING tournaments t
WHERE
    tp.tournament_key = sqlc.arg('tournamentKey')
  AND
    tp.user_id = sqlc.arg('userID')
  AND
    t.tournament_key = tp.tournament_key
  AND
    t.status = 'LOBBY'::tournament_status_enum
    RETURNING user_id;