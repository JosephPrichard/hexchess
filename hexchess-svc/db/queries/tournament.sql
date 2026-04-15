-- name: InsertTournament :one
INSERT INTO tournaments (tournament_key, name, rounds, ruleset, status, countdown, countdown_started_on, created_on, created_by, updated_on, mode)
VALUES (
    sqlc.arg('tournament_key')::uuid,
    sqlc.arg('name'),
    sqlc.arg('rounds'),
    sqlc.arg('ruleset'),
    sqlc.arg('status'),
    sqlc.arg('countdown'),
    sqlc.arg('countdown_started_on'),
    sqlc.arg('created_on'),
    sqlc.arg('created_by'),
    sqlc.arg('updated_on'),
    sqlc.arg('mode'))
RETURNING id;

-- name: InsertTournamentParticipant :exec
INSERT INTO tournament_participants (tournament_key, user_id, joined_on)
VALUES (sqlc.arg('tournament_key')::uuid, sqlc.arg('user_id'), sqlc.arg('joined_on'));

-- name: BatchInsertTournamentMatch :batchexec
INSERT INTO tournament_matches (tournament_key, game_id, white_id, black_id, round, created_on)
VALUES (
        sqlc.arg('tournament_key')::uuid,
        sqlc.arg('game_id'),
        sqlc.arg('white_id'),
        sqlc.arg('black_id'),
        sqlc.arg('round'),
        sqlc.arg('created_on'))
ON CONFLICT ON CONSTRAINT tournament_matches_pkey DO NOTHING;

-- name: UpdateTournamentStatus :exec
UPDATE tournaments
SET status = sqlc.arg('status'),
    updated_on = sqlc.arg('updated_on'),
    rounds = COALESCE(sqlc.narg('rounds'), rounds),
    winner_id = COALESCE(sqlc.narg('winner_id'), winner_id),
    countdown_started_on = COALESCE(sqlc.narg('countdown_started_on'), countdown_started_on)
WHERE tournament_key = sqlc.arg('tournament_key');

-- name: DeleteTournamentParticipant :many
DELETE FROM tournament_participants tp
USING tournaments t
WHERE
    tp.tournament_key = sqlc.arg('tournament_key') AND
    tp.user_id = sqlc.arg('user_id') AND
    t.tournament_key = tp.tournament_key AND
    t.status = 'LOBBY'::tournament_status_enum
RETURNING user_id;

-- name: SelectTournamentStatus :one
SELECT status FROM tournaments WHERE tournament_key = sqlc.arg('tournament_key')::uuid;

-- name: SelectParticipants :many
SELECT * FROM tournament_participants WHERE tournament_key = sqlc.arg('tournament_key')::uuid;

-- name: SelectMatches :many
SELECT * FROM tournament_matches WHERE tournament_key = sqlc.arg('tournament_key')::uuid;

-- name: SelectTournamentByID :one
SELECT id, tournament_key, name, rounds, ruleset, status, winner_id, countdown, countdown_started_on, created_on, updated_on, created_by, mode
FROM tournaments WHERE tournament_key = sqlc.arg('tournament_key')::uuid;

-- name: SelectParticipantsWithUserByTournamentID :many
SELECT
    tp.tournament_key,
    tp.joined_on as tournament_joined_on,
    -- maintain parity with `SelectUserWithEloByIDsRow`
    u.id as user_id,
    u.username,
    u.country,
    u.bio,
    u.joined_on as user_joined_on,
    e.elo,
    e.highest_elo,
    e.wins,
    e.losses,
    e.draws
FROM tournament_participants tp
    INNER JOIN tournaments t
        ON t.tournament_key = tp.tournament_key
    INNER JOIN users u
        ON u.id = tp.user_id
    LEFT JOIN user_mode_elos e -- must be a left join because mode elos are lazily initialized when user plays the first game.
        ON u.id = e.user_id AND e.mode = t.mode
WHERE tp.tournament_key = sqlc.arg('tournament_key')::uuid
ORDER BY tp.joined_on DESC;

-- name: SelectReplayMatchesByTournamentID :many
SELECT
    tm.ordering,
    tm.game_id,
    tm.tournament_key,
    tm.round,
    tm.created_on,
    tm.white_id,
    tm.black_id,
    r.id as replay_id,
    r.result,
    r.cause,
    r.played_on,
    r.win_elo_diff,
    r.lose_elo_diff,
    r.mode
FROM tournament_matches tm
     -- must be left join, if the game is not finished it does not have a replay yet
    LEFT JOIN replays r
        ON r.game_id = tm.game_id
WHERE tm.tournament_key = sqlc.arg('tournament_key')::uuid
ORDER BY tm.ordering DESC;

-- name: SelectTournaments :many
SELECT id, tournament_key, name, rounds, ruleset, status, winner_id, countdown, countdown_started_on, created_on, updated_on, created_by, mode
FROM tournaments
WHERE id < sqlc.arg('afterID')
ORDER BY id DESC
LIMIT sqlc.arg('perPage');

-- name: SelectTournamentsByParticipant :many
SELECT t.id, t.tournament_key, t.name, t.rounds, t.ruleset, t.status, t.winner_id, t.countdown, t.countdown_started_on, t.created_on, t.updated_on, t.created_by, t.mode
FROM tournament_participants tp
INNER JOIN tournaments t
    ON t.tournament_key = tp.tournament_key AND t.id < sqlc.arg('afterID')
WHERE tp.user_id = sqlc.arg('userID')
ORDER BY t.id DESC
LIMIT sqlc.arg('perPage');

-- name: SelectTournamentWithParticipantCountByID :one
SELECT
    t.tournament_key,
    t.rounds,
    t.ruleset,
    t.status,
    t.countdown_started_on,
    t.mode,
    (SELECT COUNT(*) FROM tournament_participants tp WHERE tp.tournament_key = t.tournament_key)::INTEGER as participant_count
FROM tournaments t
WHERE t.tournament_key = sqlc.arg('key');

-- name: SelectParticipantsForMatchmakingByTournamentID :many
SELECT tp.user_id, e.elo
FROM tournament_participants tp
    INNER JOIN tournaments t
        ON t.tournament_key = tp.tournament_key
    LEFT JOIN user_mode_elos e -- must be a left join because mode elos are lazily initialized when user plays the first game.
       ON tp.user_id = e.user_id AND e.mode = t.mode
WHERE tp.tournament_key = sqlc.arg('tournament_key')
ORDER BY tp.joined_on;

-- name: SelectMatchesByTournamentID :many
SELECT
    tm.round,
    tm.white_id,
    tm.black_id,
    r.result,
    e1.elo as white_elo,
    e2.elo as black_elo
FROM tournament_matches tm
    INNER JOIN tournaments t
        ON t.tournament_key = tm.tournament_key
    INNER JOIN replays r
        ON r.game_id = tm.game_id
    LEFT JOIN user_mode_elos e1
        ON tm.white_id = e1.user_id AND e1.mode = t.mode
    LEFT JOIN user_mode_elos e2
        ON tm.black_id = e2.user_id AND e2.mode = t.mode
WHERE tm.tournament_key = sqlc.arg('tournament_key')
ORDER BY tm.round, tm.ordering;