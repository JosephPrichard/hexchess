package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"log/slog"
	"time"
)

type Replay struct {
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
	MoveListJSON []byte
}

func InsertReplay(ctx context.Context, q *db.Queries, inst ReplayInst) error {
	count, err := q.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:  inst.WhiteID,
		BlackID:  inst.BlackID,
		Result:   inst.Result,
		Cause:    inst.Cause,
		WinElo:   inst.WinElo,
		LoseElo:  inst.LoseElo,
		MoveList: inst.MoveListJSON,
	})
	slog.Log(nil, dynLevel(err), "created a new replay", "replay", inst, "count", count, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func mapReplay(row db.GetReplayByIDRow) Replay {
	return Replay{
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

func GetReplay(ctx context.Context, q *db.Queries, id int64) (Replay, error) {
	trace := ctx.Value(TraceKey)

	row, err := q.GetReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Replay{}, ErrNoReplay
	}
	if err != nil {
		slog.Error("failed to select replay", "id", id, "err", err, "trace", trace)
		return Replay{}, fmt.Errorf("failed to get replay by id: %v", err)
	}
	replay := mapReplay(row)

	slog.Info("selected replay by id", "replay", replay, "trace", trace)
	return replay, nil
}

func GetReplayMoveList(ctx context.Context, q *db.Queries, id int64) (string, error) {
	trace := ctx.Value(TraceKey)

	moveList, err := q.GetReplayMoveList(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoReplay
	}
	if err != nil {
		slog.Error("failed to select replay move list", "id", id, "err", err, "trace", trace)
		return "", fmt.Errorf("failed to get replay move list by id: %v", err)
	}
	moveListStr := string(moveList)

	slog.Info("selected replay by id", "moveList", moveListStr, "trace", trace)
	return moveListStr, nil
}

func GetUserReplays(ctx context.Context, q *db.Queries, userID int64, afterID *int64, perPage int32) ([]Replay, error) {
	trace := ctx.Value(TraceKey)

	var pgAfterID pgtype.Int8
	if afterID != nil {
		pgAfterID.Valid = true
		pgAfterID.Int64 = *afterID
	}

	rows, err := q.GetUserReplays(ctx, db.GetUserReplaysParams{
		UserID:  userID,
		AfterID: pgAfterID,
		PerPage: perPage,
	})
	if err != nil {
		slog.Error("failed to select replays", "err", err, "trace", trace)
		return nil, fmt.Errorf("failed to get replays: %v", err)
	}

	var replays []Replay
	for _, row := range rows {
		replays = append(replays, mapReplay(db.GetReplayByIDRow(row)))
	}

	slog.Info("selected replays", "replays", replays, "userID", userID, "afterID", afterID, "perPage", perPage, "trace", trace)
	return replays, nil
}
