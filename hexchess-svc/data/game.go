package data

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"log/slog"
)

const GamesZSet = "games"

func WriteFinishedGame(ctx context.Context, stores Stores, state ChessState, isWhiteWin bool, cause ReplayCause) error {
	fail := func(m string, err error) error {
		err = fmt.Errorf("%s: %w", m, err)
		slog.ErrorContext(ctx, "failed to finish game", "err", err)
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
		slog.WarnContext(ctx, "game result update is a noop", "room", state.ID)
		return nil
	}

	slog.InfoContext(ctx, "applying ELO change set to leaderboard", "changeSet", cs, "room", state.ID)

	if err := IncrLeaderboard(ctx,
		stores.Rdb,
		UpdtLbChangeSet{ID: cs.WinID, EloDiff: cs.WinEloDiff},
		UpdtLbChangeSet{ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return fail("failed to increment user leaderboard stats", err)
	}

	slog.InfoContext(ctx, "finished game", "room", state.ID)
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

func UpdateGameResultTx(ctx context.Context, pgDB PgDB, params GRParams) (GRChangeSet, error) {
	return WithTxn(ctx, pgDB, func(q *db.Queries) (GRChangeSet, error) {
		return updateGameResult(ctx, q, params)
	})
}

func updateGameResult(ctx context.Context, q *db.Queries, params GRParams) (GRChangeSet, error) {
	result, winID, loseID := WhiteWin, params.WhiteID, params.BlackID
	if !params.IsWhiteWin {
		result, winID, loseID = BlackWin, params.BlackID, params.WhiteID
	}

	fail := func(str string, err error) (GRChangeSet, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to update stats", "winID", winID, "loseID", loseID, "err", err)
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
		slog.InfoContext(ctx, "user stats update is a noop", "winId", winID, "loseId", loseID)
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
	slog.InfoContext(ctx, "updated user stats", "inst", inst, "changeSet", cs)
	return cs, nil
}
