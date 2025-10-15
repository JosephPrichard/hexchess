package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hexchess-svc/app/util"
	"hexchess-svc/db"
	"log/slog"
	"math"
	"time"
)

type ReplayEntity struct {
	ID           int64
	WhiteID      int64
	BlackID      int64
	WhiteName    string
	BlackName    string
	WhiteCountry string
	BlackCountry string
	Result       ReplayResult
	Cause        ReplayCause
	WinElo       float64
	LoseElo      float64
	WhiteElo     float64
	BlackElo     float64
	PlayedOn     time.Time
}

type ReplayResult int

const (
	WhiteWin ReplayResult = iota
	BlackWin
	Draw
)

type ReplayCause int

const (
	Checkmate ReplayCause = iota
	Forfeit
)

type ReplayInst struct {
	WhiteID      int64   `json:"whiteId"`
	BlackID      int64   `json:"blackId"`
	Result       int32   `json:"result"`
	Cause        int32   `json:"cause"`
	WinElo       float64 `json:"winElo"`
	LoseElo      float64 `json:"loseElo"`
	MoveListJSON string
}

func InsertReplay(ctx context.Context, q *db.Queries, inst ReplayInst) (int64, error) {
	trace := ctx.Value(util.TraceKey)
	replayID, err := q.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:  inst.WhiteID,
		BlackID:  inst.BlackID,
		Result:   inst.Result,
		Cause:    inst.Cause,
		WinElo:   inst.WinElo,
		LoseElo:  inst.LoseElo,
		MoveList: []byte(inst.MoveListJSON),
	})
	if err != nil {
		slog.Error("failed to create replay", "replay", inst, "err", err, "trace", trace)
		return 0, err
	}
	
	slog.Info("created a new replay", "replay", inst, "replayID", replayID, "trace", trace)
	return replayID, nil
}

func mapReplayFromRow(row db.GetReplayByIDRow) ReplayEntity {
	return ReplayEntity{
		ID:           row.ID,
		WhiteID:      row.WhiteID,
		BlackID:      row.BlackID,
		WhiteName:    row.WhiteName,
		BlackName:    row.BlackName,
		WhiteCountry: row.WhiteCountry.String,
		BlackCountry: row.BlackCountry.String,
		Result:       ReplayResult(row.Result),
		Cause:        ReplayCause(row.Cause),
		WinElo:       row.WinElo,
		LoseElo:      row.LoseElo,
		WhiteElo:     row.WhiteElo,
		BlackElo:     row.BlackElo,
	}
}

var ErrNoReplay = errors.New("replay not found")

func GetReplay(ctx context.Context, q *db.Queries, id int64) (ReplayEntity, error) {
	trace := ctx.Value(util.TraceKey)

	row, err := q.GetReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ReplayEntity{}, ErrNoReplay
	}
	if err != nil {
		slog.Error("failed to select replay", "id", id, "err", err, "trace", trace)
		return ReplayEntity{}, fmt.Errorf("failed to get replay by id: %w", err)
	}
	replay := mapReplayFromRow(row)

	slog.Info("selected replay by id", "replay", replay, "trace", trace)
	return replay, nil
}

func GetReplayMoveList(ctx context.Context, q *db.Queries, id int64) (string, error) {
	trace := ctx.Value(util.TraceKey)

	moveList, err := q.GetReplayMoveList(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoReplay
	}
	if err != nil {
		slog.Error("failed to select replay move list", "id", id, "err", err, "trace", trace)
		return "", fmt.Errorf("failed to get replay move list by id: %w", err)
	}
	moveListStr := string(moveList)

	slog.Info("selected replay by id", "moveList", moveListStr, "trace", trace)
	return moveListStr, nil
}

func GetUserReplays(ctx context.Context, q *db.Queries, userID int64, afterID int64, perPage int32) ([]ReplayEntity, error) {
	trace := ctx.Value(util.TraceKey)

	if afterID < 0 {
		afterID = int64(math.MaxInt64)
	}

	rows, err := q.GetUserReplays(ctx, db.GetUserReplaysParams{
		UserID:  userID,
		AfterID: afterID,
		PerPage: perPage,
	})
	if err != nil {
		slog.Error("failed to select replays", "err", err, "trace", trace)
		return nil, fmt.Errorf("failed to get replays: %w", err)
	}

	var replays []ReplayEntity
	for _, row := range rows {
		replays = append(replays, mapReplayFromRow(db.GetReplayByIDRow(row)))
	}

	slog.Info("selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage, "trace", trace)
	return replays, nil
}
