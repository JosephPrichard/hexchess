package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/app/chess"
	"hexchess-svc/app/util"
	"hexchess-svc/db"
	"log/slog"
)

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
	return WithTransaction(ctx, pgDB, func(q *db.Queries) (GRChangeSet, error) {
		return UpdateGameResult(ctx, q, params)
	})
}

func UpdateGameResult(ctx context.Context, q *db.Queries, params GRParams) (GRChangeSet, error) {
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
