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

type ReplayResult string

func (r ReplayResult) IsWin() bool {
	return r == WhiteWin || r == BlackWin
}

const (
	WhiteWin ReplayResult = "WHITE_WINS"
	BlackWin ReplayResult = "BLACK_WINS"
	Draw     ReplayResult = "DRAW"
)

type ReplayCause string

const (
	Checkmate ReplayCause = "CHECKMATE"
	Forfeit   ReplayCause = "FORFEIT"
	Stalemate ReplayCause = "STALEMATE"
)

type ReplayMode string

// contains the same value as time control by coincidence, in the future we want to add the timer value to the modes
const (
	ModeUnlimited      ReplayMode = "UNLIMITED"
	ModeRealTime       ReplayMode = "REAL_TIME"
	ModeCorrespondence ReplayMode = "CORRESPONDENCE"
)

type ReplayInst struct {
	WhiteID     int64
	BlackID     int64
	Result      ReplayResult
	Cause       ReplayCause
	Mode        ReplayMode
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
		Mode:        string(inst.Mode),
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
		return nil, fmt.Errorf("select replays by user id %d: %w", userID, err)
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
)

type EloHistoriesParams struct {
	UserID    int64     `json:"userID"`
	Months    uint      `json:"months"`
	TimeUntil time.Time `json:"timeUntil"`
}

type EloHistoryBuckets []EloHistoryBucket

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

	var ehb EloHistoryBuckets
	var bd time.Duration

	playedAfter := pgtype.Timestamptz{}
	if params.Months != 0 {
		playedAfter = pgtype.Timestamptz{Valid: true, Time: params.TimeUntil.AddDate(0, -int(params.Months), 0)}
	}
	eloRows, err := dbs.Pdb.Query.GetReplayElos(ctx, db.GetReplayElosParams{
		ID:          params.UserID,
		PlayedAfter: playedAfter,
	})
	if err != nil {
		return ehb, bd, fmt.Errorf("select replay elos for user %d after %v: %w", params.UserID, playedAfter, err)
	}
	slog.InfoContext(ctx, "selected elo replay histories", "userID", params.UserID, "playedAfter", playedAfter, "eloRows", eloRows)

	ehb, bd, err = makeEloHistoryBuckets(eloRows, params)
	if err != nil {
		return ehb, bd, fmt.Errorf("make elo history buckets: %w", err)
	}
	slog.InfoContext(ctx, "retrieved elo histories", "userID", params.UserID, "eloHistoryBuckets", &ehb)
	return ehb, bd, nil
}

func makeEloHistoryBuckets(eloRows []db.GetReplayElosRow, params EloHistoriesParams) (EloHistoryBuckets, time.Duration, error) {
	eloHistoryBuckets := make(EloHistoryBuckets, 0)
	var errs []error

	if len(eloRows) == 0 {
		return eloHistoryBuckets, 0, nil
	}

	firstTime := eloRows[0].PlayedOn.Time
	lastTime := eloRows[len(eloRows)-1].PlayedOn.Time

	isLongHistory := lastTime.Sub(firstTime) > 365*24*time.Hour
	bucketDuration := ShortBucketDuration
	if isLongHistory {
		bucketDuration = LongBucketDuration
	}

	var eloTotal float64
	var eloCount int
	var prevTime time.Time

	appendBucket := func() {
		if eloCount <= 0 || prevTime.IsZero() {
			return
		}
		t := prevTime.Truncate(bucketDuration)
		eloHistoryBuckets = append(eloHistoryBuckets, EloHistoryBucket{
			Elo:       eloTotal / float64(eloCount),
			Timestamp: t.Format(time.RFC3339),
		})
	}

	accumulateRow := func(row db.GetReplayElosRow) {
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
		bucketEnd := prevTime.Add(bucketDuration)
		isExceedBucket := row.PlayedOn.Time.After(bucketEnd)
		if prevTime.IsZero() {
			eloTotal += elo
			eloCount++
			prevTime = row.PlayedOn.Time
		} else if isExceedBucket {
			appendBucket()
			eloTotal = elo
			eloCount = 1
			prevTime = row.PlayedOn.Time
		} else {
			eloTotal += elo
			eloCount++
		}
	}

	for _, row := range eloRows {
		accumulateRow(row)
	}
	appendBucket()

	return eloHistoryBuckets, bucketDuration, errors.Join(errs...)
}
