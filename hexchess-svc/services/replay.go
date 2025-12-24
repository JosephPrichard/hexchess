package svc

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

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
)

type ReplayEntity struct {
	ID           int64        `json:"id"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	WhiteName    string       `json:"whiteName"`
	BlackName    string       `json:"blackName"`
	WhiteCountry string       `json:"whiteCountry"`
	BlackCountry string       `json:"blackCountry"`
	Mode         GameMode     `json:"mode"`
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

type ReplayResult string

func (r ReplayResult) IsWin() bool {
	return r == WhiteWin || r == BlackWin
}

const (
	ResultUnknown ReplayResult = ""
	WhiteWin      ReplayResult = "WHITE_WINS"
	BlackWin      ReplayResult = "BLACK_WINS"
	Draw          ReplayResult = "DRAW"
)

var ReplayResults = []ReplayResult{WhiteWin, BlackWin, Draw}

type ReplayCause string

const (
	CauseUnknown ReplayCause = ""
	Checkmate    ReplayCause = "CHECKMATE"
	Forfeit      ReplayCause = "FORFEIT"
	Stalemate    ReplayCause = "STALEMATE"
)

var ReplayCauses = []ReplayCause{Checkmate, Forfeit, Stalemate}

type ReplayInst struct {
	WhiteID     int64
	BlackID     int64
	Result      ReplayResult
	Cause       ReplayCause
	Mode        GameMode
	WinEloDiff  float64
	LoseEloDiff float64
	// white and block elos at the time of insertion
	ReplayBlackElo     float64
	ReplayWhiteElo     float64
	PlayedOn           time.Time
	SerializedMoveHist []byte
}

func InsertReplay(ctx context.Context, query *db.Queries, inst ReplayInst) (int64, error) {
	if inst.SerializedMoveHist == nil {
		inst.SerializedMoveHist = []byte{}
	}
	playedOn := pgtype.Timestamptz{}
	if !inst.PlayedOn.IsZero() {
		playedOn = pgtype.Timestamptz{Valid: true, Time: inst.PlayedOn}
	}

	replayID, err := query.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:     inst.WhiteID,
		BlackID:     inst.BlackID,
		Result:      string(inst.Result),
		Cause:       string(inst.Cause),
		Mode:        db.ModeEnum(inst.Mode),
		WinElo:      inst.WinEloDiff,
		LoseElo:     inst.LoseEloDiff,
		WhiteElo:    inst.ReplayWhiteElo,
		BlackElo:    inst.ReplayBlackElo,
		MoveHistory: inst.SerializedMoveHist,
		PlayedOn:    playedOn,
	})
	inst.SerializedMoveHist = nil
	if err != nil {
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

func mapReplayFromRow(row db.SelectReplayByIDRow) ReplayEntity {
	replay := ReplayEntity{
		ID:           row.ID,
		WhiteID:      row.WhiteID,
		BlackID:      row.BlackID,
		WhiteName:    row.WhiteName,
		BlackName:    row.BlackName,
		WhiteCountry: row.WhiteCountry,
		BlackCountry: row.BlackCountry,
		Result:       ReplayResult(row.Result),
		Cause:        ReplayCause(row.Cause),
		Mode:         GameMode(row.Mode),
		WinEloDiff:   row.WinEloDiff,
		LoseEloDiff:  row.LoseEloDiff,
		WhiteElo:     defaultElo(row.WhiteElo),
		BlackElo:     defaultElo(row.BlackElo),
		PlayedOn:     row.PlayedOn.Time,
	}
	replay.WhiteEloDiff, replay.BlackEloDiff = getColorEloDiffs(replay.Result, replay.WinEloDiff, replay.LoseEloDiff)
	return replay
}

var ErrNoReplay = errors.New("replay not found")

func GetReplay(ctx context.Context, query *db.Queries, id int64) (ReplayEntity, error) {
	var replay ReplayEntity

	row, err := query.SelectReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return replay, ErrNoReplay
	}
	if err != nil {
		return replay, fmt.Errorf("select replay %d by id: %w", id, err)
	}
	replay = mapReplayFromRow(row)

	slog.InfoContext(ctx, "selected replay by id", "replay", replay)
	return replay, nil
}

func GetReplayMoveHistory(ctx context.Context, query *db.Queries, id int64) (*pb.MoveHistory, error) {
	b, err := query.SelectReplayMoveHistory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoReplay
	}
	if err != nil {
		return nil, fmt.Errorf("select replay %d move list by id: %w", id, err)
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

	rows, err := query.SelectUserReplays(ctx, db.SelectUserReplaysParams{
		UserID:  userID,
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("select replays by user id %d: %w", userID, err)
	}

	var replays []ReplayEntity
	for _, row := range rows {
		replays = append(replays, mapReplayFromRow(db.SelectReplayByIDRow(row)))
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

type EloHistoryBuckets map[GameMode][]EloHistoryBucket

type EloHistoryBucket struct {
	Timestamp string  `json:"timestamp"`
	Elo       float64 `json:"elo"`
}

// RetrieveEloHistoryBuckets Returns the elo replay histories for a given user organized into buckets and categorized into a map keyed by replay "mode"
// map will contain the keys "ALL" (contains data for all modes) plus all modes (ReplayModes)
func RetrieveEloHistoryBuckets(ctx context.Context, dbs *db.Databases, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	if params.TimeUntil.IsZero() {
		params.TimeUntil = time.Now()
	}
	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: params.TimeUntil.AddDate(0, -int(params.Months), 0)}
	}

	eloRows, err := dbs.Pdb.Query.SelectReplayElos(ctx, db.SelectReplayElosParams{
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

func makeEloHistoryBuckets(eloRows []db.SelectReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	buckets := make(EloHistoryBuckets)
	var errs []error

	if len(eloRows) == 0 {
		return buckets, 0, nil
	}

	firstTime := eloRows[0].PlayedOn.Time
	lastTime := eloRows[len(eloRows)-1].PlayedOn.Time

	isLongHistory := lastTime.Sub(firstTime) > 365*24*time.Hour
	bucketDuration := ShortBucketDuration
	if isLongHistory {
		bucketDuration = LongBucketDuration
	}

	type EloHistoryAcc struct {
		eloTotal float64
		eloCount int
		prevTime time.Time
		buckets  []EloHistoryBucket
	}
	type EloHistoryAccs map[GameMode]*EloHistoryAcc

	accumulators := make(EloHistoryAccs)
	for _, mode := range GameModes {
		accumulators[mode] = &EloHistoryAcc{}
	}

	appendBucketAcc := func(acc *EloHistoryAcc) {
		if acc.eloCount <= 0 || acc.prevTime.IsZero() {
			return
		}
		t := acc.prevTime.Truncate(bucketDuration)
		acc.buckets = append(acc.buckets, EloHistoryBucket{
			Elo:       acc.eloTotal / float64(acc.eloCount),
			Timestamp: t.Format(time.RFC3339),
		})
	}

	appendBucketMode := func(mode GameMode) {
		acc, ok := accumulators[mode]
		if !ok {
			errs = append(errs, fmt.Errorf("missing mode %s in elo history buckets", mode))
			return
		}
		appendBucketAcc(acc)
	}

	accumulateRow := func(row db.SelectReplayElosRow) {
		acc, ok := accumulators[GameMode(row.Mode)]
		if !ok {
			errs = append(errs, fmt.Errorf("missing mode %s in elo history buckets", row.Mode))
			return
		}
		var elo float64
		switch params.UserID {
		case row.WhiteID:
			elo = row.WhiteElo
		case row.BlackID:
			elo = row.BlackElo
		default:
			errs = append(errs, fmt.Errorf("invalid user id %d in replay elo row %v", params.UserID, row))
			return
		}
		bucketEnd := acc.prevTime.Add(bucketDuration)
		isExceedBucket := row.PlayedOn.Time.After(bucketEnd)
		if acc.prevTime.IsZero() {
			acc.eloTotal += elo
			acc.eloCount++
			acc.prevTime = row.PlayedOn.Time
		} else if isExceedBucket {
			appendBucketAcc(acc)
			acc.eloTotal = elo
			acc.eloCount = 1
			acc.prevTime = row.PlayedOn.Time
		} else {
			acc.eloTotal += elo
			acc.eloCount++
		}
	}

	for _, row := range eloRows {
		accumulateRow(row)
	}
	for _, mode := range GameModes {
		appendBucketMode(mode)
	}

	for mode, acc := range accumulators {
		if len(acc.buckets) > 0 {
			buckets[mode] = acc.buckets
		}
	}
	return buckets, bucketDuration, errors.Join(errs...)
}
