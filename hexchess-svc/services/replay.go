package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReplayDTO struct {
	// fetched from replays table directly.
	ID          int64        `json:"id"`
	WhiteID     int64        `json:"whiteId"`
	BlackID     int64        `json:"blackId"`
	Mode        GameMode     `json:"mode"`
	Result      ReplayResult `json:"result"`
	Cause       ReplayCause  `json:"cause"`
	WinEloDiff  float64      `json:"winEloDiff"`
	LoseEloDiff float64      `json:"loseEloDiff"`
	PlayedOn    time.Time    `json:"playedOn"`

	// joined from users/elo tables, may be zero if not selected.
	WhiteName    string  `json:"whiteName"`
	BlackName    string  `json:"blackName"`
	WhiteCountry string  `json:"whiteCountry"`
	BlackCountry string  `json:"blackCountry"`
	WhiteElo     float64 `json:"whiteElo"`
	BlackElo     float64 `json:"blackElo"`

	// 'view' fields that can be computed after construction.
	WhiteEloDiff float64 `json:"whiteEloDiff"`
	BlackEloDiff float64 `json:"blackEloDiff"`
}

func (replay ReplayDTO) Compute() ReplayDTO {
	switch replay.Result {
	case WhiteWin:
		replay.WhiteEloDiff, replay.BlackEloDiff = replay.WinEloDiff, replay.LoseEloDiff
	case BlackWin:
		replay.WhiteEloDiff, replay.BlackEloDiff = replay.LoseEloDiff, replay.WinEloDiff
	default:
	}
	return replay
}

func mapReplayFromByIDRow(row sqlc.SelectReplayByIDRow) (ReplayDTO, error) {
	result, err := ParseReplayResult(row.Result)
	if err != nil {
		return ReplayDTO{}, err
	}
	cause, err := ParseReplayCause(row.Cause)
	if err != nil {
		return ReplayDTO{}, err
	}
	mode, err := ParseGameMode(row.Mode)
	if err != nil {
		return ReplayDTO{}, err
	}

	return ReplayDTO{
		ID:          row.ID,
		WhiteID:     row.WhiteID.Int64,
		BlackID:     row.BlackID.Int64,
		Result:      result,
		Cause:       cause,
		Mode:        mode,
		WinEloDiff:  row.WinEloDiff,
		LoseEloDiff: row.LoseEloDiff,
		PlayedOn:    row.PlayedOn.Time,

		WhiteName:    row.WhiteName.String,
		BlackName:    row.BlackName.String,
		WhiteCountry: row.WhiteCountry.String,
		BlackCountry: row.BlackCountry.String,
		WhiteElo:     defaultElo(row.WhiteElo),
		BlackElo:     defaultElo(row.BlackElo),
	}.Compute(), nil
}

func mapReplayInst(result GameResult, changeSet GameResultChangeSet) sqlc.InsertReplayParams {
	return sqlc.InsertReplayParams{
		GameID:   result.GameID,
		WhiteID:  pgtype.Int8{Int64: result.WhiteID, Valid: IsNonGuestID(result.WhiteID)},
		BlackID:  pgtype.Int8{Int64: result.BlackID, Valid: IsNonGuestID(result.BlackID)},
		Result:   sqlc.ResultEnum(result.ReplayResult.String()),
		Cause:    sqlc.CauseEnum(result.ReplayCause.String()),
		Mode:     sqlc.ModeEnum(result.ReplayMode.String()),
		WinElo:   changeSet.WinEloDiff,
		LoseElo:  changeSet.LoseEloDiff,
		WhiteElo: changeSet.WhiteEloNext,
		BlackElo: changeSet.BlackEloNext,
		PlayedOn: pgtype.Timestamptz{Valid: true, Time: result.InsertedTime},
	}
}

var ErrNoReplay = errors.New("replay not found")

func mapGetReplayResult[ID string | int64](ctx context.Context, id ID, row sqlc.SelectReplayByIDRow, err error) (ReplayDTO, error) {
	if err != nil {
		if IsErrNoRows(err) {
			return ReplayDTO{}, ErrNoReplay
		}
		return ReplayDTO{}, fmt.Errorf("select replay %v by id: %w", id, err)
	}

	replay, err := mapReplayFromByIDRow(row)
	if err != nil {
		return ReplayDTO{}, fmt.Errorf("map replay %v from row: %w", id, err)
	}

	slog.InfoContext(ctx, "selected replay by id", "replay", replay, "id", id)
	return replay, nil
}

func (svc *Services) GetReplayByGameID(ctx context.Context, gameID string) (ReplayDTO, error) {
	row, err := svc.Querier.SelectReplayByGameID(ctx, gameID)
	return mapGetReplayResult(ctx, gameID, sqlc.SelectReplayByIDRow(row), err)
}

func (svc *Services) GetReplay(ctx context.Context, replayID int64) (ReplayDTO, error) {
	row, err := svc.Querier.SelectReplayByID(ctx, replayID)
	return mapGetReplayResult(ctx, replayID, row, err)
}

func (svc *Services) GetMovesHistory(ctx context.Context, replayID int) ([]byte, error) {
	row, err := svc.Querier.SelectReplayMoveHistories(ctx, int64(replayID))
	if err != nil {
		return nil, fmt.Errorf("select replay move histories for replay %d: %w", replayID, err)
	}
	slog.InfoContext(ctx, "selected replay move histories", "replayID", replayID)
	return row.Data, nil
}

func (svc *Services) GetUserReplays(ctx context.Context, userID int64, afterID int64, perPage int32) ([]ReplayDTO, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	rows, err := svc.Querier.SelectUserReplays(ctx, sqlc.SelectUserReplaysParams{
		UserID:  pgtype.Int8{Int64: userID, Valid: true},
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		if IsErrNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("select replays by user id %d: %w", userID, err)
	}

	var replays []ReplayDTO
	for _, row := range rows {
		replay, err := mapReplayFromByIDRow(sqlc.SelectReplayByIDRow(row))
		if err != nil {
			return nil, fmt.Errorf("map replay from row: %w", err)
		}
		replays = append(replays, replay)
	}
	slog.InfoContext(ctx, "selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage)
	return replays, nil
}

const (
	ShortBucketDuration = 24 * time.Hour
	LongBucketDuration  = 7 * 24 * time.Hour
)

type EloHistoriesParams struct {
	UserID    int64     `json:"userID"`
	Months    uint      `json:"months"`
	TimeUntil time.Time `json:"timeUntil"`
}

type EloHistoryBuckets map[string][]EloHistoryBucket

type EloHistoryBucket struct {
	Timestamp string  `json:"timestamp"`
	Elo       float64 `json:"elo"`
}

// RetrieveEloHistoryBuckets Returns the elo replay histories for a given user organized into buckets and categorized into a map keyed by replay "mode"
// map will contain the keys "ALL" (contains data for all modes) plus all modes (ReplayModes)
func (svc *Services) RetrieveEloHistoryBuckets(ctx context.Context, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	if params.TimeUntil.IsZero() {
		params.TimeUntil = time.Now()
	}
	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: params.TimeUntil.AddDate(0, -int(params.Months), 0)}
	}

	eloRows, err := svc.Querier.SelectReplayElos(ctx, sqlc.SelectReplayElosParams{
		ID:          pgtype.Int8{Int64: params.UserID, Valid: true},
		PlayedAfter: playedAfter,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("select replay elos for user %d after %v: %w", params.UserID, playedAfter, err)
	}
	slog.InfoContext(ctx, "selected elo replay histories", "userID", params.UserID, "playedAfter", playedAfter, "eloRows", eloRows)

	buckets, durations, err := makeEloHistoryBuckets(eloRows, params)
	if err != nil {
		return nil, 0, fmt.Errorf("make elo history buckets: %w", err)
	}
	slog.InfoContext(ctx, "retrieved elo histories", "userID", params.UserID, "eloHistoryBuckets", buckets)
	return buckets, durations, nil
}

const year = 365 * 24 * time.Hour

func makeEloHistoryBuckets(eloRows []sqlc.SelectReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	bucketMap := make(EloHistoryBuckets)

	if len(eloRows) == 0 {
		return bucketMap, 0, nil
	}

	firstTime := eloRows[0].PlayedOn.Time
	lastTime := eloRows[len(eloRows)-1].PlayedOn.Time

	isLongHistory := lastTime.Sub(firstTime) > year
	duration := ShortBucketDuration
	if isLongHistory {
		duration = LongBucketDuration
	}

	type Bucket struct {
		eloTotal  float64 // accumulating average.
		eloCount  int
		startTime time.Time          // begin time range of the Bucket we are accumulating.
		elements  []EloHistoryBucket // all accumulated buckets.
	}
	buckets := make(map[GameMode]*Bucket)
	for _, mode := range GameModeMap {
		buckets[mode] = &Bucket{}
	}

	// a Bucket contains the averaged data over a certain timeframe.
	appendBucket := func(bucket *Bucket) {
		if bucket.eloCount <= 0 || bucket.startTime.IsZero() {
			return
		}
		t := bucket.startTime.Truncate(duration)
		bucket.elements = append(bucket.elements, EloHistoryBucket{
			Elo:       bucket.eloTotal / float64(bucket.eloCount),
			Timestamp: t.Format(time.RFC3339),
		})
	}

	// fill buckets row by row. a Bucket is filled once the startTime is 'duration' ago relative to the current row
	for _, row := range eloRows {
		mode, err := ParseGameMode(string(row.Mode))
		if err != nil {
			return bucketMap, duration, err
		}
		bucket, ok := buckets[mode]
		if !ok {
			return bucketMap, duration, fmt.Errorf("missing mode %s in elo history buckets", row.Mode)
		}

		var elo float64
		switch params.UserID {
		case row.WhiteID.Int64:
			elo = row.WhiteElo
		case row.BlackID.Int64:
			elo = row.BlackElo
		default:
			return bucketMap, duration, fmt.Errorf("invalid user id %d in replay elo row %v", params.UserID, row)
		}

		bucketEnd := bucket.startTime.Add(duration)
		isFilledBucket := row.PlayedOn.Time.After(bucketEnd)

		if bucket.startTime.IsZero() {
			bucket.eloTotal += elo
			bucket.eloCount++
			bucket.startTime = row.PlayedOn.Time
		} else if isFilledBucket {
			appendBucket(bucket)
			bucket.eloTotal = elo
			bucket.eloCount = 1
			bucket.startTime = row.PlayedOn.Time
		} else {
			bucket.eloTotal += elo
			bucket.eloCount++
		}
	}
	// append any buckets that may not have been fully filled, but contain averaged data.
	for _, mode := range GameModeMap {
		acc, ok := buckets[mode]
		if !ok {
			return bucketMap, duration, fmt.Errorf("missing mode %s in elo history buckets", mode)
		}
		appendBucket(acc)
	}

	for mode, acc := range buckets {
		if len(acc.elements) > 0 {
			bucketMap[mode.String()] = acc.elements
		}
	}
	return bucketMap, duration, nil
}
