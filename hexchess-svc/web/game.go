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
)

func CreateGame(ctx context.Context, d data.GameplayDAL, color data.ColorSelect, timeControl data.TimeControl) (string, error) {
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

	state, err := d.SetChessState(ctx, strID, state)
	if err != nil {
		return "", err
	}

	go broadcastOnCreateGame(d)
	return strID, nil
}

func broadcastOnCreateGame(a data.GameplayDAL) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in CreateGame broadcast handler", "err", r)
		}
	}()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "create-game-broadcast-handler")

	count, err := a.GetChessStateCount(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count chess states after creating game", "err", err)
		return
	}
	slog.InfoContext(ctx, "counted chess states after creating game", "count", count)

	//data.GameBroadcaster.MultiCaster(count)
}

func JoinGame(ctx context.Context, d data.GameplayDAL, gameID string, player data.PlayerState) (data.ChessState, error) {

	state, err := d.GetChessState(ctx, gameID)
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

	state, err = d.SetChessState(ctx, gameID, state)

	logs.DynLog(ctx, "player joined game", err, "playerID", player.ID, "stateID", gameID, "color", color, "err", err)
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

	state, err := d.GetChessState(ctx, gameID)
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
		if err := d.WriteFinishedGame(ctx, state, isWhiteWin, data.Checkmate); err != nil {
			return MoveResult{}, err
		}
	}

	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "state", state)

	state, err = d.SetChessState(ctx, gameID, state)
	if err != nil {
		return MoveResult{}, err
	}

	return MoveResult{Room: state, Move: move}, nil
}

func ForfeitGame(ctx context.Context, d data.GameplayDAL, gameID string, player data.PlayerState) error {

	state, err := d.GetChessState(ctx, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID)
		return nil
	}

	didBlackForfeit := state.BlackPlayer.ID == player.ID
	state.IsEnded = true

	if err := d.WriteFinishedGame(ctx, state, didBlackForfeit, data.Forfeit); err != nil {
		return err
	}
	if _, err := d.SetChessState(ctx, gameID, state); err != nil {
		return err
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID, "err", err)
	return err
}
