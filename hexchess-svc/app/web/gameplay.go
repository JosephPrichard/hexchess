package web

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/app/chess"
	"hexchess-svc/app/data"
	"hexchess-svc/app/util"
	"log/slog"
	"math/big"
)

const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func CreateGame(ctx context.Context, a data.GameplayDAL, color data.ColorSelect, timeControl data.TimeControl) (string, error) {
	bID := make([]byte, 8)
	for i := range bID {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			return "", fmt.Errorf("error generating id: %w", err)
		}
		bID[i] = characters[n.Int64()]
	}
	strID := string(bID)

	state := data.MakeStartChessState(strID, timeControl)
	state.FirstColor = color
	state.Game.InitPieceMoves()

	slog.Info("created chess game", "state", state, "trace", ctx.Value(util.TraceKey))

	state, err := a.SetChessState(ctx, strID, state)
	if err != nil {
		return "", err
	}

	go broadcastOnCreateGame(a)
	return strID, nil
}

func broadcastOnCreateGame(a data.GameplayDAL) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in CreateGame broadcast handler", "err", r)
		}
	}()

	ctx := context.WithValue(context.Background(), util.TraceKey, "create-game-broadcast-handler")

	count, err := a.GetChessStateCount(ctx)
	if err != nil {
		slog.Error("failed to count chess states after creating game", "err", err)
		return
	}
	slog.Info("counted chess states after creating game", "count", count)

	//data.GameBroadcaster.MultiBroker(count)
}

func JoinGame(ctx context.Context, a data.GameplayDAL, gameID string, player data.PlayerState) (data.ChessState, error) {
	trace := ctx.Value(util.TraceKey)

	state, err := a.GetChessState(ctx, gameID)
	if err != nil {
		return data.ChessState{}, err
	}

	hasWhite := state.WhitePlayer != nil
	hasBlack := state.BlackPlayer != nil
	playerExists := (hasWhite && state.WhitePlayer.ID == player.ID) || (hasBlack && state.BlackPlayer.ID == player.ID)

	color := "none"
	if !playerExists {
		switch {
		case !hasWhite && !hasBlack:
			n, err := rand.Int(rand.Reader, big.NewInt(1000))
			if err != nil {
				return data.ChessState{}, err
			}
			pickWhite := state.FirstColor == data.Random && n.Int64()%2 == 0 || state.FirstColor == data.White
			if pickWhite {
				state.WhitePlayer = &player
				color = "white"
			} else {
				state.BlackPlayer = &player
				color = "black"
			}
		case !hasBlack:
			state.BlackPlayer = &player
			color = "black"
		case !hasWhite:
			state.WhitePlayer = &player
			color = "white"
		default:
			return state, nil
		}
	}

	state, err = a.SetChessState(ctx, gameID, state)

	util.DynLog("player joined game", err, "playerID", player.ID, "stateID", gameID, "color", color, "err", err, "trace", trace)
	return state, err
}

type MoveResult struct {
	Room data.ChessState
	Move chess.PieceMove
}

var (
	ErrFinishedGame = errors.New("move attempted on finished game")
	ErrTurn         = errors.New("not player's turn")
	ErrInvalidMove  = errors.New("invalid move")
)

func MakeGameMove(ctx context.Context, d data.GameplayDAL, gameID string, player data.PlayerState, move chess.PieceMove) (MoveResult, error) {
	trace := ctx.Value(util.TraceKey)

	state, err := d.GetChessState(ctx, gameID)
	if err != nil {
		return MoveResult{}, err
	}

	game := state.Game
	board := game.Board
	currPlayer := state.CurrPlayer()

	if state.IsEnded {
		slog.Info("make move: attempted on ended game", "gameId", gameID)
		return MoveResult{}, ErrFinishedGame
	}
	if currPlayer == nil || *currPlayer != player {
		slog.Info("make move: invalid turn", "player", player.ID, "game", gameID)
		return MoveResult{}, ErrTurn
	}
	if !game.IsValidMove(move) {
		slog.Info("make move: invalid move", "player", player.ID, "move", move, "game", gameID)
		return MoveResult{}, ErrInvalidMove
	}

	move = game.MakeMove(move.From, move.To)
	game.InitPieceMoves()
	state.MoveList = append(state.MoveList, move)

	if game.CheckmateReached() {
		state.IsEnded = true
		isWhiteWin := !board.IsWhiteTurn
		if err := handleFinishGame(ctx, d, state, isWhiteWin, data.Checkmate); err != nil {
			return MoveResult{}, err
		}
	}

	slog.Info("made move on game", "player", player.ID, "move", move, "state", state, "trace", trace)

	state, err = d.SetChessState(ctx, gameID, state)
	if err != nil {
		return MoveResult{}, err
	}

	return MoveResult{Room: state, Move: move}, nil
}

func ForfeitGame(ctx context.Context, d data.GameplayDAL, gameID string, player data.PlayerState) error {
	trace := ctx.Value(util.TraceKey)

	state, err := d.GetChessState(ctx, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID, "trace", trace)
		return nil
	}

	didBlackForfeit := state.BlackPlayer.ID == player.ID
	state.IsEnded = true

	if err := handleFinishGame(ctx, d, state, didBlackForfeit, data.Forfeit); err != nil {
		return err
	}
	if _, err := d.SetChessState(ctx, gameID, state); err != nil {
		return err
	}

	slog.Info("player forfeited game", "playerID", player.ID, "gameId", gameID, "err", err, "trace", trace)
	return err
}

func handleFinishGame(ctx context.Context, d data.GameplayDAL, state data.ChessState, isWhiteWin bool, cause data.ReplayCause) error {
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

	params := data.GRParams{
		WhiteID:    whiteID,
		BlackID:    blackID,
		Cause:      cause,
		IsWhiteWin: isWhiteWin,
		MoveList:   state.MoveList,
	}
	cs, err := d.UpdateGameResult(ctx, params)
	if err != nil {
		return fail("failed to execute finish game tx", err)
	}
	if cs == (data.GRChangeSet{}) {
		return nil
	}

	slog.Info("applying ELO change set to leaderboard", "changeSet", cs, "room", state.ID, "trace", trace)

	if err := d.UpdateLeaderboard(ctx,
		data.UpdtLbChangeSet{ID: cs.WinID, EloDiff: cs.WinEloDiff},
		data.UpdtLbChangeSet{ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return fail("failed to increment user leaderboard stats", err)
	}

	slog.Info("finished game", "room", state.ID, "trace", trace)
	return nil
}
