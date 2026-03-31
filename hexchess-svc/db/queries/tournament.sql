-- name: InsertTournament :one
INSERT INTO tournaments (tournament_key, name, depth, status, scheduled_on, created_on, created_by, updated_on, mode)
VALUES (
    sqlc.arg('tournament_key'),
    sqlc.arg('name'),
    sqlc.arg('depth'),
    sqlc.arg('status'),
    sqlc.arg('scheduled_on'),
    sqlc.arg('created_on'),
    sqlc.arg('created_by'),
    sqlc.arg('updated_on'),
    sqlc.arg('mode'))
RETURNING id;

-- name: SelectTournamentById :one
SELECT id, name, tournament_key, depth, status, scheduled_on, created_on, updated_on, created_by, mode
FROM tournaments WHERE id = sqlc.arg('id');

-- name: SelectParticipantsByTournamentId :many
SELECT
    tp.tournament_id,
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
        ON t.id = tp.tournament_id
    INNER JOIN users u
        ON u.id = tp.user_id
    LEFT JOIN user_mode_elos e -- must be a left join because mode elos are lazily initialized when user plays the first game.
        ON u.id = e.user_id AND e.mode = t.mode
WHERE tournament_id = sqlc.arg('tournament_id')
ORDER BY tp.joined_on DESC;

-- name: SelectMatchesByTournamentId :many
SELECT
    tm.id as tournament_match_id,
    tm.game_id,
    tm.tournament_id,
    tm.depth,
    tm.created_on,
    r.id as replay_id,
    r.white_id,
    r.black_id,
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
WHERE tournament_id = sqlc.arg('tournament_id')
ORDER BY tm.created_on DESC;

-- name: SelectTournaments :many
SELECT id, name, tournament_key, depth, status, scheduled_on, created_on, updated_on, created_by, mode
FROM tournaments
WHERE id < sqlc.arg('afterID')
ORDER BY id DESC
LIMIT sqlc.arg('perPage');

-- name: SelectTournamentsByParticipant :many
SELECT t.id, t.name, t.tournament_key, t.depth, t.status, t.scheduled_on, t.created_on, t.updated_on, t.created_by, t.mode
FROM tournament_participants tp
INNER JOIN tournaments t
    ON t.id = tp.tournament_id AND t.id < sqlc.arg('afterID')
WHERE tp.user_id = sqlc.arg('userID')
ORDER BY t.id DESC
LIMIT sqlc.arg('perPage');