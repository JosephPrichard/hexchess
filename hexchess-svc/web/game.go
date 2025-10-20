package web

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"log/slog"
	"math/big"
	"strconv"
)

func CreateGame(ctx context.Context, stores data.Stores, color data.ColorSelect, timeControl data.TimeControl) (string, error) {
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

	state := data.MakeStartChessState(strID, timeControl)
	state.FirstColor = color
	state.Game.InitPieceMoves()

	slog.InfoContext(ctx, "created chess game", "state", state, "trace", ctx.Value(logs.TraceKey))

	state, err := data.SetChessState(ctx, stores.Rdb, strID, state)
	if err != nil {
		return "", err
	}

	go broadcastOnCreateGame(stores)
	return strID, nil
}

func broadcastOnCreateGame(stores data.Stores) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in create game broadcast handler", "err", r)
		}
	}()
	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-game-broadcast-handler")

	count, err := data.GetChessStateCount(ctx, stores.Rdb)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count chess states after creating game", "err", err)
		return
	}
	if err := data.BroadcastMessage(ctx, stores.Rdb, stores.Rdb.GamesChan, strconv.AppendInt(nil, count, 10)); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast chess states count after creating game", "err", err)
		return
	}
	slog.InfoContext(ctx, "counted chess states after creating game", "count", count)
}

func JoinGame(ctx context.Context, stores data.Stores, gameID string, player data.PlayerState) (data.ChessState, error) {
	state, err := data.GetChessState(ctx, stores.Rdb, gameID)
	if err != nil {
		return data.ChessState{}, err
	}

	hasWhite := state.WhitePlayer != nil
	hasBlack := state.BlackPlayer != nil
	playerExists := (hasWhite && state.WhitePlayer.ID == player.ID) || (hasBlack && state.BlackPlayer.ID == player.ID)

	if playerExists {
		slog.WarnContext(ctx, "player has already joined game", "playerID", player.ID, "state", state)
		return state, nil
	}
	if !hasWhite && !hasBlack {
		n, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			return data.ChessState{}, err
		}
		pickWhite := state.FirstColor == data.Random && n.Int64()%2 == 0 || state.FirstColor == data.White
		if pickWhite {
			state.WhitePlayer = &player
		} else {
			state.BlackPlayer = &player
		}
	} else if !hasBlack {
		state.BlackPlayer = &player
	} else if !hasWhite {
		state.WhitePlayer = &player
	} else {
		return state, nil
	}

	state, err = data.SetChessState(ctx, stores.Rdb, gameID, state)
	logs.DynLog(ctx, "player joined game", err, "playerID", player.ID, "state", state, "err", err)
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

func MakeGameMove(ctx context.Context, stores data.Stores, gameID string, player data.PlayerState, move chess.PieceMove) (MoveResult, error) {
	state, err := data.GetChessState(ctx, stores.Rdb, gameID)
	if err != nil {
		return MoveResult{}, err
	}

	game := &state.Game
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

	move = game.MakeMove(move.From, move.To)
	game.InitPieceMoves()
	state.MoveList = append(state.MoveList, move)

	if game.CheckmateReached() {
		state.IsEnded = true
		isWhiteWin := !game.Board.IsWhiteTurn
		if err := data.WriteFinishedGame(ctx, stores, state, isWhiteWin, data.Checkmate); err != nil {
			return MoveResult{}, err
		}
	}

	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "state", state)

	state, err = data.SetChessState(ctx, stores.Rdb, gameID, state)
	if err != nil {
		return MoveResult{}, err
	}

	return MoveResult{Room: state, Move: move}, nil
}

func ForfeitGame(ctx context.Context, stores data.Stores, gameID string, player data.PlayerState) error {
	state, err := data.GetChessState(ctx, stores.Rdb, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID)
		return nil
	}

	didBlackForfeit := state.BlackPlayer.ID == player.ID
	state.IsEnded = true

	if err := data.WriteFinishedGame(ctx, stores, state, didBlackForfeit, data.Forfeit); err != nil {
		return err
	}
	if _, err := data.SetChessState(ctx, stores.Rdb, gameID, state); err != nil {
		return err
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID, "err", err)
	return err
}
