package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"log/slog"
	"math"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
)

var ReplayModes = []string{"UNLIMITED", "REAL_TIME", "CORRESPONDENCE"}

type ReplayEntity struct {
	ID           int64        `json:"id"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	WhiteName    string       `json:"whiteName"`
	BlackName    string       `json:"blackName"`
	WhiteCountry string       `json:"whiteCountry"`
	BlackCountry string       `json:"blackCountry"`
	Result       ReplayResult `json:"result"`
	Cause        ReplayCause  `json:"cause"`
	WinEloDiff   float64      `json:"winEloDiff"`
	LoseEloDiff  float64      `json:"loseEloDiff"`
	WhiteElo     float64      `json:"whiteElo"`
	BlackElo     float64      `json:"blackElo"`
	WhiteEloDiff float64      `json:"whiteEloDiff"`
	BlackEloDiff float64      `json:"blackEloDiff"`
	PlayedOn     time.Time    `json:"playedOn"`
}

var ReplayEntityCmpOpts = cmpopts.IgnoreFields(ReplayEntity{}, "PlayedOn")
var ReplayRowCmpOpts = cmpopts.IgnoreFields(db.Replay{}, "PlayedOn")

type ReplayResult string

const (
	WhiteWin ReplayResult = "WHITE_WINS"
	BlackWin ReplayResult = "BLACK_WINS"
	Draw     ReplayResult = "DRAW"
)

type ReplayCause string

const (
	Checkmate ReplayCause = "CHECKMATE"
	Forfeit   ReplayCause = "FORFEIT"
)

type ReplayMode string

// contains the same value as time control by coincidence, in the future we want to add the timer value to the modes
const (
	ModeUnlimited      ReplayMode = "UNLIMITED"
	ModeRealTime       ReplayMode = "REAL_TIME"
	ModeCorrespondence ReplayMode = "CORRESPONDENCE"
)

type ReplayInst struct {
	WhiteID          int64
	BlackID          int64
	Result           ReplayResult
	Cause            ReplayCause
	Mode             ReplayMode
	WinEloDiff       float64
	LoseEloDiff      float64
	BlackElo         float64
	WhiteElo         float64
	MoveHistoryProto []byte
	PlayedOn         time.Time
}

func InsertReplay(ctx context.Context, query *db.Queries, inst ReplayInst) (int64, error) {
	if inst.MoveHistoryProto == nil {
		inst.MoveHistoryProto = []byte{}
	}
	playedOn := pgtype.Timestamptz{}
	if !inst.PlayedOn.IsZero() {
		playedOn = pgtype.Timestamptz{Valid: true, Time: inst.PlayedOn}
	}
	whiteElo := pgtype.Float8{}
	if inst.WhiteElo != 0 {
		whiteElo = pgtype.Float8{Valid: true, Float64: inst.WhiteElo}
	}
	blackElo := pgtype.Float8{}
	if inst.BlackElo != 0 {
		blackElo = pgtype.Float8{Valid: true, Float64: inst.BlackElo}
	}

	replayID, err := query.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:     inst.WhiteID,
		BlackID:     inst.BlackID,
		Result:      string(inst.Result),
		Cause:       string(inst.Cause),
		Mode:        string(inst.Mode),
		WinElo:      inst.WinEloDiff,
		LoseElo:     inst.LoseEloDiff,
		WhiteElo:    whiteElo,
		BlackElo:    blackElo,
		MoveHistory: inst.MoveHistoryProto,
		PlayedOn:    playedOn,
	})
	inst.MoveHistoryProto = nil
	if err != nil {
		slog.ErrorContext(ctx, "failed to create replay", "replay", inst, "err", err)
		return 0, err
	}
	slog.InfoContext(ctx, "created a new replay", "replay", inst, "replayID", replayID)
	return replayID, nil
}

func getColorEloDiffs(result ReplayResult, winEloDiff float64, loseEloDiff float64) (float64, float64) {
	var whiteEloDiff, blackEloDiff float64
	switch result {
	case WhiteWin:
		whiteEloDiff, blackEloDiff = winEloDiff, loseEloDiff
	case BlackWin:
		whiteEloDiff, blackEloDiff = loseEloDiff, winEloDiff
	default:
	}
	return whiteEloDiff, blackEloDiff
}

func mapReplayFromRow(row db.GetReplayByIDRow) (ReplayEntity, error) {
	replay := ReplayEntity{
		ID:           row.ID,
		WhiteID:      row.WhiteID,
		BlackID:      row.BlackID,
		WhiteName:    row.WhiteName,
		BlackName:    row.BlackName,
		WhiteCountry: row.WhiteCountry.String,
		BlackCountry: row.BlackCountry.String,
		Result:       ReplayResult(row.Result),
		Cause:        ReplayCause(row.Cause),
		WinEloDiff:   row.WinEloDiff,
		LoseEloDiff:  row.LoseEloDiff,
		WhiteElo:     row.WhiteElo,
		BlackElo:     row.BlackElo,
		PlayedOn:     row.PlayedOn.Time,
	}
	replay.WhiteEloDiff, replay.BlackEloDiff = getColorEloDiffs(replay.Result, replay.WinEloDiff, replay.LoseEloDiff)
	return replay, nil
}

var ErrNoReplay = errors.New("replay not found")

func GetReplay(ctx context.Context, query *db.Queries, id int64) (ReplayEntity, error) {
	var replay ReplayEntity

	row, err := query.GetReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return replay, ErrNoReplay
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to select replay", "id", id, "err", err)
		return replay, fmt.Errorf("get replay %d by id: %w", id, err)
	}
	if replay, err = mapReplayFromRow(row); err != nil {
		return replay, err
	}

	slog.InfoContext(ctx, "selected replay by id", "replay", replay)
	return replay, nil
}

func GetReplayMoveHistory(ctx context.Context, query *db.Queries, id int64) (*pb.MoveHistory, error) {
	b, err := query.GetReplayMoveHistory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoReplay
	}
	if err != nil {
		return nil, fmt.Errorf("get replay %d move list by id: %w", id, err)
	}
	var moveHist pb.MoveHistory
	if err := proto.Unmarshal(b, &moveHist); err != nil {
		return nil, fmt.Errorf("unmarshal move history: %w", err)
	}
	return &moveHist, nil
}

func GetUserReplays(ctx context.Context, query *db.Queries, userID int64, afterID int64, perPage int32) ([]ReplayEntity, error) {
	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	rows, err := query.GetUserReplays(ctx, db.GetUserReplaysParams{
		UserID:  userID,
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to select replays", "userID", userID, "afterID", afterID, "err", err)
		return nil, fmt.Errorf("select replays: %w", err)
	}

	var replays []ReplayEntity
	for _, row := range rows {
		replay, err := mapReplayFromRow(db.GetReplayByIDRow(row))
		if err != nil {
			return nil, err
		}
		replays = append(replays, replay)
	}
	slog.InfoContext(ctx, "selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage)
	return replays, nil
}

const (
	ShortBucketDuration = 24 * time.Hour
	LongBucketDuration  = 7 * 24 * time.Hour
	LongMonthsThreshold = 12
)

type EloHistoriesParams struct {
	UserID int64 `json:"userID"`
	Months int   `json:"months"`
}

type EloHistoryBuckets map[string][]EloHistoryBucket

type EloHistoryBucket struct {
	Timestamp string  `json:"timestamp"`
	Elo       float64 `json:"elo"`
}

// RetrieveEloHistoryBuckets Returns the elo replay histories for a given user organized into buckets and categorized into a map keyed by replay "mode"
// map will contain the keys "ALL" (contains data for all modes) plus all modes (ReplayModes)
func RetrieveEloHistoryBuckets(ctx context.Context, dbs *Databases, timeUntil time.Time, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	var ehb EloHistoryBuckets
	var bd time.Duration

	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: timeUntil.AddDate(0, -params.Months, 0)}
	}
	eloRows, err := dbs.Pdb.Query.GetReplayElos(ctx, db.GetReplayElosParams{
		ID:          params.UserID,
		PlayedAfter: playedAfter,
	})
	if err != nil {
		return ehb, bd, fmt.Errorf("select replay elos: %w", err)
	}
	slog.InfoContext(ctx, "selected elo replay histories", "userID", params.UserID, "playedAfter", playedAfter, "eloRows", eloRows)

	if ehb, bd, err = makeEloHistoryBuckets(eloRows, params); err != nil {
		return ehb, bd, fmt.Errorf("make elo history buckets: %w", err)
	}
	slog.InfoContext(ctx, "retrieved elo histories", "userID", params.UserID, "eloHistoryBuckets", &ehb)
	return ehb, bd, nil
}

func makeEloHistoryBuckets(eloRows []db.GetReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	eloHistoryBuckets := make(EloHistoryBuckets)
	var errs []error

	if len(eloRows) == 0 {
		return eloHistoryBuckets, 0, nil
	}

	bucketDuration := ShortBucketDuration
	if params.Months >= LongMonthsThreshold || params.Months == 0 {
		bucketDuration = LongBucketDuration
	}

	type bucketAcc struct {
		buckets  []EloHistoryBucket
		eloTotal float64
		eloCount int
		prevTime time.Time
	}

	bucketAccumulators := make(map[string]*bucketAcc)
	bucketAccumulators["ALL"] = &bucketAcc{}
	for _, mode := range ReplayModes {
		bucketAccumulators[mode] = &bucketAcc{buckets: make([]EloHistoryBucket, 0)}
	}

	appendBucket := func(mode string) {
		acc, ok := bucketAccumulators[mode]
		if !ok {
			errs = append(errs, fmt.Errorf("invalid mode '%s' while accumulating elo history bucket", mode))
			return
		}
		if acc.eloCount <= 0 || acc.prevTime.IsZero() {
			return
		}
		t := acc.prevTime.Truncate(bucketDuration)
		acc.buckets = append(acc.buckets, EloHistoryBucket{
			Elo:       acc.eloTotal / float64(acc.eloCount),
			Timestamp: t.Format(time.RFC3339),
		})
	}

	accumulateRow := func(row db.GetReplayElosRow, mode string) {
		acc, ok := bucketAccumulators[mode]
		if !ok {
			errs = append(errs, fmt.Errorf("invalid mode '%s' while accumulating elo history row: %+v", mode, row))
			return
		}
		bucketEnd := acc.prevTime.Add(bucketDuration)
		isExceedBucket := row.PlayedOn.Time.After(bucketEnd)
		var elo float64
		switch params.UserID {
		case row.WhiteID:
			elo = row.WhiteElo
		case row.BlackID:
			elo = row.BlackElo
		}
		if acc.prevTime.IsZero() {
			acc.eloTotal += elo
			acc.eloCount++
			acc.prevTime = row.PlayedOn.Time
		} else if isExceedBucket {
			appendBucket(mode)
			acc.eloTotal = elo
			acc.eloCount = 1
			acc.prevTime = row.PlayedOn.Time
		} else {
			acc.eloTotal += elo
			acc.eloCount++
		}
	}

	for _, row := range eloRows {
		accumulateRow(row, "ALL")
		accumulateRow(row, row.Mode)
	}
	appendBucket("ALL")
	for _, mode := range ReplayModes {
		appendBucket(mode)
	}

	for mode, acc := range bucketAccumulators {
		eloHistoryBuckets[mode] = acc.buckets
	}
	return eloHistoryBuckets, bucketDuration, errors.Join(errs...)
}
