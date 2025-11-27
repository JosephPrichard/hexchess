package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"log/slog"
	"math"
	"time"
)

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
	WinElo       float64      `json:"winElo"`
	LoseElo      float64      `json:"loseElo"`
	WhiteElo     float64      `json:"whiteElo"`
	BlackElo     float64      `json:"blackElo"`
	WhiteEloDiff float64      `json:"whiteEloDiff"`
	BlackEloDiff float64      `json:"blackEloDiff"`
	PlayedOn     time.Time    `json:"playedOn"`
}

var ReplayEntityCmpOpts = cmpopts.IgnoreFields(ReplayEntity{}, "PlayedOn")

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

type ReplayInst struct {
	WhiteID          int64        `json:"whiteId"`
	BlackID          int64        `json:"blackId"`
	Result           ReplayResult `json:"result"`
	Cause            ReplayCause  `json:"cause"`
	WinElo           float64      `json:"winElo"`
	LoseElo          float64      `json:"loseElo"`
	MoveHistoryProto []byte
}

func InsertReplay(ctx context.Context, query *db.Queries, inst ReplayInst) (int64, error) {
	if inst.MoveHistoryProto == nil {
		inst.MoveHistoryProto = []byte{}
	}

	replayID, err := query.InsertReplay(ctx, db.InsertReplayParams{
		WhiteID:     inst.WhiteID,
		BlackID:     inst.BlackID,
		Result:      string(inst.Result),
		Cause:       string(inst.Cause),
		WinElo:      inst.WinElo,
		LoseElo:     inst.LoseElo,
		MoveHistory: inst.MoveHistoryProto,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create replay", "replay", inst, "err", err)
		return 0, err
	}

	inst.MoveHistoryProto = nil
	slog.InfoContext(ctx, "created a new replay", "replay", inst, "replayID", replayID)
	return replayID, nil
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
		WinElo:       row.WinElo,
		LoseElo:      row.LoseElo,
		WhiteElo:     row.WhiteElo,
		BlackElo:     row.BlackElo,
		PlayedOn:     row.PlayedOn.Time,
	}
	switch replay.Result {
	case WhiteWin:
		replay.WhiteEloDiff, replay.BlackEloDiff = row.WinElo, row.LoseElo
	case BlackWin:
		replay.WhiteEloDiff, replay.BlackEloDiff = row.LoseElo, row.WinElo
	default:
	}
	return replay, nil
}

var ErrNoReplay = errors.New("replay not found")

func GetReplay(ctx context.Context, query *db.Queries, id int64) (ReplayEntity, error) {
	row, err := query.GetReplayByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ReplayEntity{}, ErrNoReplay
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to select replay", "id", id, "err", err)
		return ReplayEntity{}, fmt.Errorf("failed to get replay %d by id: %w", id, err)
	}
	replay, err := mapReplayFromRow(row)
	if err != nil {
		return ReplayEntity{}, err
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
		return nil, fmt.Errorf("failed to get replay %d move list by id: %w", id, err)
	}

	var moveHist pb.MoveHistory
	if err := proto.Unmarshal(b, &moveHist); err != nil {
		return nil, fmt.Errorf("failed to unmarshal move history: %w", err)
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
		return nil, fmt.Errorf("failed to select replays: %w", err)
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
