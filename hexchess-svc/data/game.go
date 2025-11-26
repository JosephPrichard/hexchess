package data

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log/slog"
	"math/big"
)

const GamesZSet = "games"

func CreateGame(ctx context.Context, rdb Redis, color ColorSelect, timeControl TimeControl, initialBoard *chess.Board) (string, error) {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	bID := make([]byte, 8)
	for i := range bID {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			return "", fmt.Errorf("error generating game id: %w", err)
		}
		bID[i] = characters[n.Int64()]
	}
	strID := string(bID)

	state := MakeState(strID, timeControl)
	state.FirstColor = color
	state.Game.InitPieceMoves()
	if initialBoard != nil {
		state.Game.Board = *initialBoard
	}

	slog.InfoContext(ctx, "created chess game", "state", state)

	state, err := SetChessState(ctx, rdb, strID, state)
	if err != nil {
		return "", err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Warn("recovered in panic while broadcasting game event", "err", r)
			}
		}()
		ctx := context.WithValue(context.Background(), util.Trace, "create-game-broadcast-handler")

		count, err := GetChessStateCount(ctx, rdb)
		if err != nil {
			slog.ErrorContext(ctx, "failed to count chess states after creating game", "err", err)
			return
		}
		if err := BroadcastGameCount(ctx, rdb, count, strID); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast chess states count after creating game", "err", err)
			return
		}
		slog.InfoContext(ctx, "counted chess states after creating game", "count", count)
	}()
	return strID, nil
}

func JoinGame(ctx context.Context, rdb Redis, gameID string, player *PlayerState) (ChessState, error) {
	state, err := GetChessState(ctx, rdb, gameID)
	if err != nil {
		return ChessState{}, err
	}

	if player == nil {
		slog.WarnContext(ctx, "unprovided player did not join game", "state", state)
		return state, nil
	}

	var playerExists bool
	if state.WhitePlayer == nil && state.BlackPlayer == nil {
		n, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			return ChessState{}, fmt.Errorf("failed to generate randint used to select first color: %w", err)
		}
		pickWhite := state.FirstColor == Random && n.Int64()%2 == 0 || state.FirstColor == White
		if pickWhite {
			state.WhitePlayer = player
		} else {
			state.BlackPlayer = player
		}
	} else if state.BlackPlayer != nil && state.WhitePlayer == nil {
		if state.BlackPlayer.ID == player.ID {
			playerExists = true
		} else {
			state.WhitePlayer = player
		}
	} else if state.WhitePlayer != nil && state.BlackPlayer == nil {
		if state.WhitePlayer.ID == player.ID {
			playerExists = true
		} else {
			state.BlackPlayer = player
		}
	}

	if playerExists {
		slog.WarnContext(ctx, "player has already joined game", "playerID", player.ID, "state", state)
		return state, nil
	}

	state, err = SetChessState(ctx, rdb, gameID, state)
	util.DynLog(ctx, "player joined game", err, "playerID", player.ID, "state", state, "err", err)
	return state, err
}

type MoveResult struct {
	Room ChessState
	Move chess.HistMove
}

var (
	ErrFinishedGame = errors.New("move attempted on finished game")
	ErrTurn         = errors.New("not player's turn")
	ErrInvalidMove  = errors.New("invalid move")
)

func DoMakeMove(ctx context.Context, state ChessState, player PlayerState, move chess.PieceMove) (MoveResult, error) {
	game := &state.Game
	gameID := state.ID
	currPlayer := state.CurrPlayer()

	if state.IsEnded {
		slog.InfoContext(ctx, "make move: attempted on ended game", "gameId", gameID)
		return MoveResult{}, ErrFinishedGame
	}
	if currPlayer == nil || *currPlayer != player {
		slog.InfoContext(ctx, "make move: invalid turn", "player", player.ID, "game", gameID)
		return MoveResult{}, ErrTurn
	}
	if !game.IsValidMove(move) {
		slog.InfoContext(ctx, "make move: invalid move", "player", player.ID, "move", move, "game", gameID)
		return MoveResult{}, ErrInvalidMove
	}

	hm := game.MakeMove(move.From, move.To)
	game.InitPieceMoves()

	if game.Checkmate() {
		state.IsEnded = true
	}
	return MoveResult{Room: state, Move: hm}, nil
}

func MakeGameMove(ctx context.Context, stores Stores, gameID string, player PlayerState, move chess.PieceMove) (MoveResult, error) {
	state, err := GetChessState(ctx, stores.Rdb, gameID)
	if err != nil {
		return MoveResult{}, err
	}

	mr, err := DoMakeMove(ctx, state, player, move)
	if err != nil {
		return MoveResult{}, err
	}

	if state.IsEnded {
		isWhiteWin := !state.Game.Board.IsWhiteTurn
		if err := WriteFinishedGame(ctx, stores, state, isWhiteWin, Checkmate); err != nil {
			return MoveResult{}, err
		}
	}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "state", state)

	state, err = SetChessState(ctx, stores.Rdb, gameID, state)
	if err != nil {
		return MoveResult{}, err
	}
	return mr, nil
}

func ForfeitGame(ctx context.Context, stores Stores, gameID string, player PlayerState) error {
	state, err := GetChessState(ctx, stores.Rdb, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID)
		return nil
	}

	didBlackForfeit := state.BlackPlayer.ID == player.ID
	state.IsEnded = true

	if _, err := SetChessState(ctx, stores.Rdb, gameID, state); err != nil {
		return err
	}
	if err := WriteFinishedGame(ctx, stores, state, didBlackForfeit, Forfeit); err != nil {
		return err
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID, "err", err)
	return err
}

func WriteFinishedGame(ctx context.Context, stores Stores, state ChessState, isWhiteWin bool, cause ReplayCause) error {
	fail := func(m string, err error) error {
		err = fmt.Errorf("%s: %w", m, err)
		slog.ErrorContext(ctx, "failed to finish game", "err", err)
		return err
	}

	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		return fmt.Errorf("assertion error: room players must not be nil on a finished game: roomID: %s", state.ID)
	}
	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	moveHistBytes, err := chess.MarshalMoveHistory(state.InitialBoard, state.Game.Moves)
	if err != nil {
		return fail("failed to marshal move history", err)
	}
	cs, err := UpdateGameResultTx(ctx, stores.PgDB, GRParams{
		WhiteID:          whiteID,
		BlackID:          blackID,
		Cause:            cause,
		IsWhiteWin:       isWhiteWin,
		MoveHistoryProto: moveHistBytes,
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
	WhiteID          int64       `json:"whiteId"`
	BlackID          int64       `json:"blackId"`
	Cause            ReplayCause `json:"cause"`
	IsWhiteWin       bool        `json:"isWhiteWin"`
	MoveHistoryProto []byte
}

type GRChangeSet struct {
	ReplayID    int64
	WinID       int64
	LoseID      int64
	WinEloDiff  float64
	LoseEloDiff float64
}

func UpdateGameResultTx(ctx context.Context, pgDB PgDB, params GRParams) (GRChangeSet, error) {
	return WithTxn(TxnArgs[GRChangeSet]{
		Ctx:  ctx,
		PgDB: pgDB,
		TxFn: func(q *db.Queries) (GRChangeSet, error) {
			return updateGameResult(ctx, q, params)
		},
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

	inst := ReplayInst{
		WhiteID:          params.WhiteID,
		BlackID:          params.BlackID,
		Result:           result,
		Cause:            params.Cause,
		WinElo:           winEloDiff,
		LoseElo:          loseEloDiff,
		MoveHistoryProto: params.MoveHistoryProto,
	}
	replayID, err := InsertReplay(ctx, q, inst)
	if err != nil {
		return fail("failed to insert replay", err)
	}

	cs := GRChangeSet{ReplayID: replayID, WinID: winID, LoseID: loseID, WinEloDiff: winEloDiff, LoseEloDiff: loseEloDiff}
	slog.InfoContext(ctx, "updated user stats", "inst", inst, "changeSet", cs)
	return cs, nil
}
