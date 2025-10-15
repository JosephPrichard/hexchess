package data

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/app/chess"
	"hexchess-svc/app/util"
	"hexchess-svc/db"
	"log/slog"
)

func WriteFinishedGame(ctx context.Context, stores Stores, state ChessState, isWhiteWin bool, cause ReplayCause) error {
	trace := ctx.Value(util.TraceKey)
	fail := func(m string, err error) error {
		err = fmt.Errorf("%s: %v", m, err)
		slog.Error("failed to finish game", "err", err, "trace", trace)
		return err
	}

	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		panic(fmt.Errorf("assertion error: room players must not be nil: roomID: %s", state.ID))
	}
	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	cs, err := UpdateGameResultTx(ctx, stores.PgDB, GRParams{
		WhiteID:    whiteID,
		BlackID:    blackID,
		Cause:      cause,
		IsWhiteWin: isWhiteWin,
		MoveList:   state.MoveList,
	})
	if err != nil {
		return fail("failed to execute finish game tx", err)
	}
	if cs == (GRChangeSet{}) {
		slog.Warn("game result update is a noop", "room", state.ID, "trace", trace)
		return nil
	}

	slog.Info("applying ELO change set to leaderboard", "changeSet", cs, "room", state.ID, "trace", trace)

	if err := IncrLeaderboard(ctx,
		stores.Rdb,
		UpdtLbChangeSet{ID: cs.WinID, EloDiff: cs.WinEloDiff},
		UpdtLbChangeSet{ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return fail("failed to increment user leaderboard stats", err)
	}

	slog.Info("finished game", "room", state.ID, "trace", trace)
	return nil
}

type GRParams struct {
	WhiteID    int64       `json:"whiteId"`
	BlackID    int64       `json:"blackId"`
	Cause      ReplayCause `json:"cause"`
	IsWhiteWin bool        `json:"isWhiteWin"`
	MoveList   []chess.PieceMove
}

type GRChangeSet struct {
	ReplayID    int64
	WinID       int64
	LoseID      int64
	WinEloDiff  float64
	LoseEloDiff float64
}

func UpdateGameResultTx(ctx context.Context, pgDB DB, params GRParams) (GRChangeSet, error) {
	return WithTransaction(ctx, pgDB, func(q *db.Queries) (GRChangeSet, error) { return updateGameResult(ctx, q, params) })
}

func updateGameResult(ctx context.Context, q *db.Queries, params GRParams) (GRChangeSet, error) {
	result, winID, loseID := WhiteWin, params.WhiteID, params.BlackID
	if !params.IsWhiteWin {
		result, winID, loseID = BlackWin, params.BlackID, params.WhiteID
	}

	trace := ctx.Value(util.TraceKey)
	fail := func(str string, err error) (GRChangeSet, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to update stats", "winID", winID, "loseID", loseID, "err", err, "trace", trace)
		return GRChangeSet{}, err
	}

	winElo, err := q.GetElo(ctx, winID)
	if err != nil {
		return fail("failed to get winner elo", err)
	}
	loseElo, err := q.GetElo(ctx, loseID)
	if err != nil {
		return fail("failed to get loser elo", err)
	}

	winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
	loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))
	winEloDiff := winEloNext - winElo
	loseEloDiff := loseEloNext - loseElo

	if winEloDiff == 0 && loseEloDiff == 0 {
		slog.Info("user stats update is a noop", "winId", winID, "loseId", loseID, "trace", trace)
		return GRChangeSet{}, nil
	}

	if err := q.UpdateWins(ctx, db.UpdateWinsParams{ID: winID, Elo: winEloNext}); err != nil {
		return fail("failed to update win elo", err)
	}
	if err := q.UpdateLosses(ctx, db.UpdateLossesParams{ID: loseID, Elo: loseEloNext}); err != nil {
		return fail("failed to update lose elo", err)
	}

	moveListJson, err := json.Marshal(params.MoveList)
	if err != nil {
		return fail("failed to unmarshal move list", err)
	}
	inst := ReplayInst{
		WhiteID:      params.WhiteID,
		BlackID:      params.BlackID,
		Result:       int32(result),
		Cause:        int32(params.Cause),
		WinElo:       winEloDiff,
		LoseElo:      loseEloDiff,
		MoveListJSON: string(moveListJson),
	}
	replayID, err := InsertReplay(ctx, q, inst)
	if err != nil {
		return fail("failed to insert replay", err)
	}

	cs := GRChangeSet{ReplayID: replayID, WinID: winID, LoseID: loseID, WinEloDiff: winEloDiff, LoseEloDiff: loseEloDiff}
	slog.Info("updated user stats", "inst", inst, "changeSet", cs, "trace", trace)
	return cs, nil
}

//go:generate mockgen -source game.go -destination game_mock.go -package data
type GameplayDAL interface {
	GetChessStateCount(ctx context.Context) (int64, error)
	GetChessState(ctx context.Context, id string) (ChessState, error)
	SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error)
	WriteFinishedGame(ctx context.Context, state ChessState, isWhiteWin bool, cause ReplayCause) error
}

type GameplayDao struct {
	Stores
}

func (d *GameplayDao) GetChessStateCount(ctx context.Context) (int64, error) {
	return GetChessStateCount(ctx, d.Rdb)
}

func (d *GameplayDao) GetChessState(ctx context.Context, id string) (ChessState, error) {
	return GetChessState(ctx, d.Rdb, id)
}

func (d *GameplayDao) SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error) {
	return SetChessState(ctx, d.Rdb, id, state)
}

func (d *GameplayDao) WriteFinishedGame(ctx context.Context, state ChessState, isWhiteWin bool, cause ReplayCause) error {
	return WriteFinishedGame(ctx, d.Stores, state, isWhiteWin, cause)
}
