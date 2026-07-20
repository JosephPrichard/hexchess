-- name: InsertReplay :one
INSERT INTO replays (game_id,
                     white_id,
                     black_id,
                     result,
                     cause,
                     win_elo_diff,
                     lose_elo_diff,
                     white_elo,
                     black_elo,
                     played_on,
                     mode,
                     turn_count)
VALUES (sqlc.arg('gameID'),
        sqlc.arg('whiteID'),
        sqlc.arg('blackID'),
        sqlc.arg('result'),
        sqlc.arg('cause'),
        sqlc.arg('winElo'),
        sqlc.arg('loseElo'),
        sqlc.arg('whiteElo'),
        sqlc.arg('blackElo'),
        COALESCE(sqlc.narg('playedOn'), CURRENT_TIMESTAMP)::TIMESTAMPTZ,
        sqlc.arg('mode'),
        sqlc.arg('turnCount')) RETURNING id;

-- name: UpsertReplayMoveHistories :exec
INSERT INTO replay_move_histories (replay_id, data)
VALUES (sqlc.arg('replayID'), sqlc.arg('data')) ON CONFLICT
ON CONSTRAINT replay_move_histories_pkey
    DO
UPDATE
    SET data = sqlc.arg('data');

-- name: SelectReplayMoveHistoriesByIDs :many
SELECT *
FROM replay_move_histories
WHERE replay_id = ANY (sqlc.arg('replayIDs')::bigint[]);

-- name: SelectReplayMoveHistoryByID :one
SELECT *
FROM replay_move_histories
WHERE replay_id = sqlc.arg('replayID')
ORDER BY replay_id DESC LIMIT 1;

-- name: SelectReplayRowByID :one
SELECT *
FROM replays r
WHERE r.id = sqlc.arg('id');

-- name: SelectReplayByID :one
SELECT r.id,
       r.white_id,
       r.black_id,
       r.result,
       r.cause,
       r.played_on,
       r.win_elo_diff,
       r.lose_elo_diff,
       r.mode,
       r.rating,
       r.turn_count,
       u1.username AS white_name,
       u1.country  AS white_country,
       e1.elo      AS white_elo,
       u2.username AS black_name,
       u2.country  AS black_country,
       e2.elo      AS black_elo
FROM replays r
         -- ensures we get the white/black elo for mode at the time of retrieval
         LEFT JOIN users u1
                   ON u1.id = r.white_id
         LEFT JOIN users u2
                   ON u2.id = r.black_id
         LEFT JOIN user_mode_elos e1
                   ON e1.user_id = r.white_id AND e1.mode = r.mode
         LEFT JOIN user_mode_elos e2
                   ON e2.user_id = r.black_id AND e2.mode = r.mode
WHERE r.id = sqlc.arg('id');

-- name: SelectReplayByGameID :one
SELECT r.id,
       r.white_id,
       r.black_id,
       r.result,
       r.cause,
       r.played_on,
       r.win_elo_diff,
       r.lose_elo_diff,
       r.mode,
       r.rating,
       r.turn_count,
       u1.username AS white_name,
       u1.country  AS white_country,
       e1.elo      AS white_elo,
       u2.username AS black_name,
       u2.country  AS black_country,
       e2.elo      AS black_elo
FROM replays r
         -- ensures we get the white/black elo for mode at the time of retrieval
         LEFT JOIN users u1
                   ON u1.id = r.white_id
         LEFT JOIN users u2
                   ON u2.id = r.black_id
         LEFT JOIN user_mode_elos e1
                   ON e1.user_id = r.white_id AND e1.mode = r.mode
         LEFT JOIN user_mode_elos e2
                   ON e2.user_id = r.black_id AND e2.mode = r.mode
WHERE r.game_id = sqlc.arg('game_id');

-- name: SelectReplayElos :many
SELECT id,
       mode,
       played_on,
       white_id,
       black_id,
       white_elo,
       black_elo -- gets the white/black elo at the time of insertion
FROM replays
WHERE (white_id = sqlc.arg('id') OR black_id = sqlc.arg('id'))
  AND (played_on > sqlc.narg('playedAfter') OR sqlc.narg('playedAfter') IS NULL)
ORDER BY played_on;

-- name: SelectReplaysByQuery :many
SELECT r.id,
       r.white_id,
       r.black_id,
       r.result,
       r.cause,
       r.played_on,
       r.win_elo_diff,
       r.lose_elo_diff,
       r.mode,
       r.rating,
       r.turn_count,
       u1.username AS white_name,
       u1.country  AS white_country,
       e1.elo      AS white_elo,
       u2.username AS black_name,
       u2.country  AS black_country,
       e2.elo      AS black_elo
FROM replays r
         -- ensures we get the white/black elo for mode at the time of retrieval
         LEFT JOIN users u1
                   ON u1.id = r.white_id
         LEFT JOIN users u2
                   ON u2.id = r.black_id
         LEFT JOIN user_mode_elos e1
                   ON e1.user_id = r.white_id AND e1.mode = r.mode
         LEFT JOIN user_mode_elos e2
                   ON e2.user_id = r.black_id AND e2.mode = r.mode
WHERE (r.mode = sqlc.narg('mode') OR sqlc.narg('mode') IS NULL)
    AND (r.result = sqlc.narg('result') OR sqlc.narg('result') IS NULL)
    AND (r.cause = sqlc.narg('cause') OR sqlc.narg('cause') IS NULL)
    AND (r.white_id = sqlc.narg('whiteID') OR sqlc.narg('whiteID') IS NULL)
    AND (r.black_id = sqlc.narg('blackID') OR sqlc.narg('blackID') IS NULL)
    AND (
        r.white_id = sqlc.narg('userID') OR r.black_id = sqlc.narg('userID') OR
        sqlc.narg('userID') IS NULL
    )
    AND (
        (r.white_id = sqlc.arg('winnerID') AND r.result = 'WHITE_WINS') OR
        (r.black_id = sqlc.arg('winnerID') AND r.result = 'BLACK_WINS') OR
        sqlc.narg('winnerID') IS NULL
    )
    AND (
        (r.white_id = sqlc.arg('loserID') AND r.result = 'BLACK_WINS') OR
        (r.black_id = sqlc.arg('loserID') AND r.result = 'WHITE_WINS') OR
        sqlc.narg('loserID') IS NULL
    )
    AND (
        sqlc.narg('fromDateDays')::INT IS NULL OR
            r.played_on_as_days >= sqlc.narg('fromDateDays')::INT
    )
    AND (
        sqlc.narg('toDateDays')::INT IS NULL OR
            r.played_on_as_days <= sqlc.narg('toDateDays')::INT
    )
    AND
    CASE
        WHEN sqlc.arg('sortKey')::TEXT = 'turnCount'
        THEN r.turn_count < sqlc.arg('afterTurnCount') OR
              (r.turn_count = sqlc.arg('afterTurnCount') AND r.id < sqlc.arg('afterID'))
        WHEN sqlc.arg('sortKey')::TEXT = 'rating'
        THEN r.rating < sqlc.arg('afterRating') OR
              (r.rating = sqlc.arg('afterRating') AND r.id < sqlc.arg('afterID'))
        ELSE
          r.id < sqlc.arg('afterID')
    END
ORDER BY
    CASE
        WHEN sqlc.arg('sortKey')::TEXT = 'turnCount'
        THEN r.turn_count
    END DESC,
    CASE
        WHEN sqlc.arg('sortKey')::TEXT = 'rating'
        THEN r.rating
    END DESC,
    r.id DESC LIMIT sqlc.arg('perPage');

-- name: SelectReplaysExistsByIDs :many
SELECT id
FROM replays
WHERE id = ANY (sqlc.arg('ids')::bigint[]);

-- name: SelectReplayIDByGameID :one
SELECT id
FROM replays
WHERE game_id = sqlc.arg('gameID');