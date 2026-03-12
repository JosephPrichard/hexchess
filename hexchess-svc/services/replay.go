package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"hexchess-svc/db"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReplayEntity struct {
	ID           int64     `json:"id"`
	WhiteID      int64     `json:"whiteId"`
	BlackID      int64     `json:"blackId"`
	WhiteName    string    `json:"whiteName"`
	BlackName    string    `json:"blackName"`
	WhiteCountry string    `json:"whiteCountry"`
	BlackCountry string    `json:"blackCountry"`
	Mode         string    `json:"mode"`
	Result       string    `json:"result"`
	Cause        string    `json:"cause"`
	WinEloDiff   float64   `json:"winEloDiff"`
	LoseEloDiff  float64   `json:"loseEloDiff"`
	WhiteElo     float64   `json:"whiteElo"`
	BlackElo     float64   `json:"blackElo"`
	WhiteEloDiff float64   `json:"whiteEloDiff"`
	BlackEloDiff float64   `json:"blackEloDiff"`
	PlayedOn     time.Time `json:"playedOn"`
}

type ReplayInst struct {
	GameID      string
	WhiteID     int64
	BlackID     int64
	Result      ReplayResult
	Cause       ReplayCause
	Mode        GameMode
	WinEloDiff  float64
	LoseEloDiff float64
	// replay move history blob
	MoveHistBlob []byte
	// white and block elos at the time of insertion
	ReplayBlackElo float64
	ReplayWhiteElo float64
	PlayedOn       time.Time
}

// insertReplay A replay is only ever inserted as part of a game result transaction to ensure data consistency
func insertReplay(ctx context.Context, query *db.Queries, inst ReplayInst) (int64, error) {
	if inst.MoveHistBlob == nil {
		inst.MoveHistBlob = []byte{}
	}

	playedOn := pgtype.Timestamptz{}
	if !inst.PlayedOn.IsZero() {
		playedOn = pgtype.Timestamptz{Valid: true, Time: inst.PlayedOn}
	}

	replayID, err := query.InsertReplay(ctx, db.InsertReplayParams{
		GameID:   inst.GameID,
		WhiteID:  inst.WhiteID,
		BlackID:  inst.BlackID,
		Result:   db.ResultEnum(inst.Result.String()),
		Cause:    db.CauseEnum(inst.Cause.String()),
		Mode:     db.ModeEnum(inst.Mode.String()),
		WinElo:   inst.WinEloDiff,
		LoseElo:  inst.LoseEloDiff,
		WhiteElo: inst.ReplayWhiteElo,
		BlackElo: inst.ReplayBlackElo,
		PlayedOn: playedOn,
	})
	if err != nil {
		return 0, err
	}
	if err = query.InsertReplayMoveHistories(ctx, db.InsertReplayMoveHistoriesParams{
		ReplayID: replayID,
		Data:     inst.MoveHistBlob,
	}); err != nil {
		return 0, err
	}
	slog.InfoContext(ctx, "created a new replay", "replay", inst, "replayID", replayID)
	return replayID, nil
}

func mapReplayFromRow(row db.SelectReplayByIDRow) (ReplayEntity, error) {
	replay := ReplayEntity{
		ID:           row.ID,
		WhiteID:      row.WhiteID,
		BlackID:      row.BlackID,
		WhiteName:    row.WhiteName,
		BlackName:    row.BlackName,
		WhiteCountry: row.WhiteCountry,
		BlackCountry: row.BlackCountry,
		Result:       string(row.Result),
		Cause:        string(row.Cause),
		Mode:         string(row.Mode),
		WinEloDiff:   row.WinEloDiff,
		LoseEloDiff:  row.LoseEloDiff,
		WhiteElo:     defaultElo(row.WhiteElo),
		BlackElo:     defaultElo(row.BlackElo),
		PlayedOn:     row.PlayedOn.Time,
	}
	switch replay.Result {
	case WhiteWin.String():
		replay.WhiteEloDiff, replay.BlackEloDiff = replay.WinEloDiff, replay.LoseEloDiff
	case BlackWin.String():
		replay.WhiteEloDiff, replay.BlackEloDiff = replay.LoseEloDiff, replay.WinEloDiff
	default:
	}
	return replay, nil
}

var ErrNoReplay = errors.New("replay not found")

func (svc *Services) GetReplay(ctx context.Context, id int64) (ReplayEntity, error) {
	var replay ReplayEntity

	row, err := svc.Query().SelectReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return replay, ErrNoReplay
	}
	if err != nil {
		return replay, fmt.Errorf("select replay %d by id: %w", id, err)
	}
	replay, err = mapReplayFromRow(row)
	if err != nil {
		return replay, fmt.Errorf("map replay %d by id: %w", id, err)
	}

	slog.InfoContext(ctx, "selected replay by id", "replay", replay)
	return replay, nil
}

func (svc *Services) GetMovesHistory(ctx context.Context, replayID int) ([]byte, error) {
	row, err := svc.Query().SelectReplayMoveHistories(ctx, int64(replayID))
	if err != nil {
		return nil, fmt.Errorf("select replay move histories for replay %d: %w", replayID, err)
	}
	return row.Data, nil
}

func (svc *Services) GetUserReplays(ctx context.Context, userID int64, afterID int64, perPage int32) ([]ReplayEntity, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	rows, err := svc.Query().SelectUserReplays(ctx, db.SelectUserReplaysParams{
		UserID:  userID,
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("select replays by user id %d: %w", userID, err)
	}

	var replays []ReplayEntity
	for _, row := range rows {
		replay, err := mapReplayFromRow(db.SelectReplayByIDRow(row))
		if err != nil {
			return nil, fmt.Errorf("map replay %d by id: %w", row.ID, err)
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

	eloRows, err := svc.Query().SelectReplayElos(ctx, db.SelectReplayElosParams{
		ID:          params.UserID,
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

func makeEloHistoryBuckets(eloRows []db.SelectReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
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
		case row.WhiteID:
			elo = row.WhiteElo
		case row.BlackID:
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
