package svc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/chess"

	"hexchess-svc/internal/logutil"
	"hexchess-svc/model"
	"log/slog"
	"math/big"

	"github.com/redis/go-redis/v9"
)

var ErrForfeitPlayer = errors.New("must be a player to forfeit or abort")

type ErrStartedGame struct {
	GameID string
}

func (e ErrStartedGame) Error() string {
	return fmt.Sprintf("does not have both players (gameId=%s)", e.GameID)
}

type ErrFinishedGame struct {
	GameID string
}

func (e ErrFinishedGame) Error() string {
	return fmt.Sprintf("attempted on ended game gameId=%s", e.GameID)
}

type ErrTurn struct {
	GameID   string
	PlayerID int64
	CurrID   int64
}

func (e ErrTurn) Error() string {
	return fmt.Sprintf("invalid turn (player=%d, curr=%d, game=%s)", e.PlayerID, e.CurrID, e.GameID)
}

type ErrInvalidMove struct {
	GameID    string
	PlayerID  int64
	Violation error
}

func (e ErrInvalidMove) Error() string {
	return fmt.Sprintf("invalid move (violation=%v, player=%d, game=%s)", e.Violation, e.PlayerID, e.GameID)
}

func MakeGameID() string {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	bytesID := make([]byte, 8)
	for i := range bytesID {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			panic("failed to generate random number: " + err.Error())
		}
		bytesID[i] = characters[n.Int64()]
	}
	return string(bytesID)
}

func (svc *HexchessServices) CreateGame(ctx context.Context, color model.GameColor, mode model.GameMode, initialBoard *chess.Board) (string, error) {
	strID := MakeGameID()

	state := model.MakeChessState(model.StateSetup{ID: strID, Mode: mode, FirstColor: color, InitialBoard: initialBoard})
	state.Game.InitPieceMoves()

	slog.InfoContext(ctx, "created chess game", "chessMeta", state.ChessMeta)

	if err := svc.SetChessState(ctx, strID, state); err != nil {
		return "", fmt.Errorf("set chess state by existingID %s: %w", strID, err)
	}

	go func() {
		if err := svc.broadcastGameCounts(strID); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast game count after creating game", "error", err)
		}
	}()
	return strID, nil
}

func (svc *HexchessServices) broadcastGameCounts(strID string) error {
	ctx := context.WithValue(context.Background(), logutil.Trace, "broadcastGameCounts:"+strID)

	count, err := svc.GetChessStateCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to count chess game after creating game: %w", err)
	}
	if err := svc.broadcaster.BroadcastGameCount(ctx, count); err != nil {
		return fmt.Errorf("failed to broadcast chess game count after creating game: %w", err)
	}
	slog.InfoContext(ctx, "counted games after creating game", "count", count)
	return nil
}

func (svc *HexchessServices) JoinGame(ctx context.Context, gameID string, player model.PlayerState) (*model.ChessState, error) {
	update := func(state *model.ChessState) error {
		if !player.Present {
			slog.WarnContext(ctx, "player did not join the game", "playerID", player.ID)
			return nil
		}

		var playerExists bool
		if !state.WhitePlayer.Present && !state.BlackPlayer.Present {
			n, err := rand.Int(rand.Reader, big.NewInt(1000))
			if err != nil {
				return fmt.Errorf("generate randint used to select first color: %w", err)
			}
			pickWhite := state.FirstColor == model.Random && n.Int64()%2 == 0 || state.FirstColor == model.White
			if pickWhite {
				state.WhitePlayer = player
			} else {
				state.BlackPlayer = player
			}
		} else if state.BlackPlayer.Present && !state.WhitePlayer.Present {
			if state.BlackPlayer.ID == player.ID {
				playerExists = true
			} else {
				state.WhitePlayer = player
			}
		} else if state.WhitePlayer.Present && !state.BlackPlayer.Present {
			if state.WhitePlayer.ID == player.ID {
				playerExists = true
			} else {
				state.BlackPlayer = player
			}
		}
		if playerExists {
			slog.WarnContext(ctx, "player has already joined game", "playerID", player.ID, "chessMeta", state.ChessMeta)
			return nil
		}
		return nil
	}
	state, err := svc.updateChessStateTxn(ctx, gameID, update, nil)
	if state != nil {
		slog.InfoContext(ctx, "player joined game", "playerID", player.ID, "chessMeta", state.ChessMeta)
	}
	return state, err
}

type MoveResult struct {
	State *model.ChessState
	Move  chess.HistMove
}

func (svc *HexchessServices) MakeGameMove(ctx context.Context, gameID string, player model.PlayerState, move chess.Move) (MoveResult, error) {
	update := func(state *model.ChessState) error {
		// pre move validations on chess state
		if !state.HasBothPlayers() {
			return ErrStartedGame{GameID: gameID}
		}
		if state.EndState.IsEnded() {
			return ErrFinishedGame{GameID: gameID}
		}
		currPlayer := state.CurrPlayer()
		if !currPlayer.Present || currPlayer.ID != player.ID {
			return ErrTurn{GameID: gameID, PlayerID: player.ID, CurrID: currPlayer.ID}
		}

		// perform move validations then move
		state.Game.EnsurePieceMoves()
		if _, err := state.Game.MakeValidMove(move); err != nil {
			return ErrInvalidMove{GameID: gameID, PlayerID: player.ID, Violation: err}
		}

		// post move state mutations and processing
		if state.Game.Checkmate() {
			state.EndState = model.Finished
		}
		state.UndoState = model.UndoState{}
		state.Game.ClearTables()

		return nil
	}
	commit := func(pipe redis.Pipeliner, state *model.ChessState) error {
		if state.EndState.IsEnded() {
			slog.InfoContext(ctx, "game has reached checkmate", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

			result := model.WhiteWin
			if state.Game.Board.IsWhiteTurn {
				result = model.BlackWin
			}
			if err := svc.sendFinishGameEvent(ctx, pipe, model.FinishedGame{
				GameID:       gameID,
				WhitePlayer:  state.WhitePlayer,
				BlackPlayer:  state.BlackPlayer,
				ReplayMode:   state.Mode,
				ReplayResult: result,
				ReplayCause:  model.Checkmate,
			}); err != nil {
				return fmt.Errorf("push finished game event: %w", err)
			}
		}
		return nil
	}
	state, err := svc.updateChessStateTxn(ctx, gameID, update, commit)
	if err != nil {
		return MoveResult{}, err
	}

	histMove := state.Game.LastMove() // invariant: if this function does not error before this line, it will have at least one move.

	moveResult := MoveResult{State: state, Move: histMove}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "moveResult", moveResult, "move", move, "chessMeta", state.ChessMeta)
	return moveResult, nil
}

var (
	ErrUndoNoop       = errors.New("no undo to perform")
	ErrUndoCurrPlayer = errors.New("must not be the current player to undo a move")
	ErrNoUndo         = errors.New("no undo to accept or reject")
)

type UndoKind int

const (
	UndoCreate UndoKind = iota
	UndoAccept
	UndoReject
)

func (svc *HexchessServices) AttemptGameUndo(ctx context.Context, gameID string, player model.PlayerState, kind UndoKind) (*model.ChessState, error) {
	update := func(state *model.ChessState) error {
		switch kind {
		case UndoCreate:
			if player.IsSame(state.CurrPlayer()) {
				return ErrUndoCurrPlayer
			}
			state.UndoID = player.ID
		case UndoAccept:
			if state.UndoID == 0 {
				return ErrNoUndo
			}
			if state.UndoID != player.ID {
				if err := state.Undo(); err != nil {
					return err
				}
				state.UndoState = model.UndoState{}
			} else {
				return ErrUndoNoop
			}
		case UndoReject:
			if state.UndoID == 0 {
				return ErrNoUndo
			}
			state.UndoState = model.UndoState{}
		}
		return nil
	}
	state, err := svc.updateChessStateTxn(ctx, gameID, update, nil)
	if state != nil {
		slog.InfoContext(ctx, "attempt game undo", "playerID", player.ID, "chessMeta", state.ChessMeta)
	}
	return state, err
}

func (svc *HexchessServices) EndGame(ctx context.Context, gameID string, player model.PlayerState) (model.EndKind, error) {
	update := func(state *model.ChessState) error {
		if state.EndState.IsEnded() {
			return ErrFinishedGame{GameID: gameID}
		}
		if !state.IsEitherPlayer(player) {
			return ErrForfeitPlayer
		}
		if state.EndState.IsEnded() {
			return ErrFinishedGame{GameID: gameID}
		}

		isForfeit := state.WhitePlayer.Present && state.BlackPlayer.Present

		if isForfeit {
			state.EndState = model.Finished
		} else {
			state.EndState = model.Aborted
		}
		return nil
	}
	commit := func(pipe redis.Pipeliner, state *model.ChessState) error {
		if state.EndState == model.Finished {
			result := model.BlackWin
			if state.BlackPlayer.ID == player.ID {
				result = model.WhiteWin
			}

			if err := svc.sendFinishGameEvent(ctx, pipe, model.FinishedGame{
				GameID:       gameID,
				WhitePlayer:  state.WhitePlayer,
				BlackPlayer:  state.BlackPlayer,
				ReplayMode:   state.Mode,
				ReplayResult: result,
				ReplayCause:  model.Forfeit,
			}); err != nil {
				return fmt.Errorf("push finished game event: %w", err)
			}
		}
		return nil
	}
	state, err := svc.updateChessStateTxn(ctx, gameID, update, commit)
	if err != nil {
		return model.NotEnded, err
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID)
	return state.EndState, nil
}
