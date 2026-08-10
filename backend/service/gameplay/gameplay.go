package gameplay

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/serrors"
	"time"

	"hexchess-svc/model"
	"log/slog"
	"math/big"

	"github.com/redis/go-redis/v9"
)

type GamePlayService struct {
	redis      cache.Redis
	chessState ChessStateMutator
}

type ChessStateMutator interface {
	SetChessStatePiped(ctx context.Context, setter gamestate.RedisChessSetter, id model.GameID, state *model.ChessState, updtTime time.Time) error
	UpdateChessStateTxn(ctx context.Context, gameID model.GameID, update gamestate.ChessUpdateFn, commit gamestate.ChessCommitFn) (*model.ChessState, error)
}

func NewGameplayService(redis cache.Redis, chessState ChessStateMutator) *GamePlayService {
	return &GamePlayService{redis: redis, chessState: chessState}
}

var ErrForfeitPlayer = errors.New("must be a player to forfeit or abort")

type ErrStartedGame struct {
	GameID model.GameID
}

func (e ErrStartedGame) Error() string {
	return fmt.Sprintf("does not have both players (gameId=%s)", e.GameID)
}

type ErrFinishedGame struct {
	GameID model.GameID
}

func (e ErrFinishedGame) Error() string {
	return fmt.Sprintf("attempted on ended game gameId=%s", e.GameID)
}

type ErrTurn struct {
	GameID   model.GameID
	PlayerID int64
	CurrID   int64
}

func (e ErrTurn) Error() string {
	return fmt.Sprintf("invalid turn (player=%d, curr=%d, game=%s)", e.PlayerID, e.CurrID, e.GameID)
}

type ErrInvalidMove struct {
	GameID    model.GameID
	PlayerID  int64
	Violation error
}

func (e ErrInvalidMove) Error() string {
	return fmt.Sprintf("invalid move (violation=%v, player=%d, game=%s)", e.Violation, e.PlayerID, e.GameID)
}

func mapMetadataUpdt(state *model.ChessState) model.GameMetadataUpdt {
	return model.GameMetadataUpdt{
		GameID:      state.ID,
		WhitePlayer: opt.Option[int64]{Value: state.WhitePlayer.ID, Present: state.WhitePlayer.Present},
		BlackPlayer: opt.Option[int64]{Value: state.BlackPlayer.ID, Present: state.BlackPlayer.Present},
		Mode:        state.Mode,
	}
}

func (services *GamePlayService) CreateGame(ctx context.Context, color model.GameColor, mode model.GameMode, initialBoard *chess.Board) (model.GameID, error) {
	gameID := model.NewGameID()
	err := services.SetupGame(ctx, model.StateSetup{ID: gameID, Mode: mode, FirstColor: color, InitialBoard: initialBoard})
	return gameID, err
}

func (services *GamePlayService) SetupGame(ctx context.Context, setup model.StateSetup) error {
	gameID := setup.ID

	state := model.NewChessState(setup)
	state.Game.InitPieceMoves()

	_, err := services.redis.PrimaryClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {

		if err := services.chessState.SetChessStatePiped(ctx, pipe, gameID, state, time.Now()); err != nil {
			return serrors.New("set chess state", err, "gameID", gameID)
		}
		return producers.ProduceUpdtGameMetadata(ctx, pipe, mapMetadataUpdt(state))
	})
	return err
}

func (services *GamePlayService) JoinGame(ctx context.Context, gameID model.GameID, player model.PlayerState) (*model.ChessState, error) {
	update := func(state *model.ChessState) error {
		slog.InfoContext(ctx, "updating game state by joining", "player", player.ID, "gameID", gameID)

		if !player.Present {
			slog.WarnContext(ctx, "player did not join the game", "playerID", player.ID)
			return nil
		}

		var playerExists bool
		if !state.WhitePlayer.Present && !state.BlackPlayer.Present {
			n, err := rand.Int(rand.Reader, big.NewInt(1000))
			if err != nil {
				return serrors.New("generate randint used to select first color", err)
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
			slog.WarnContext(ctx, "player has already joined game", "playerID", player.ID, "gameID", gameID)
			return nil
		}
		return nil
	}
	commit := func(pipe redis.Pipeliner, state *model.ChessState) error {
		slog.InfoContext(ctx, "committing game state when joining", "player", player.ID, "gameID", gameID)
		return producers.ProduceUpdtGameMetadata(ctx, pipe, mapMetadataUpdt(state))
	}
	state, err := services.chessState.UpdateChessStateTxn(ctx, gameID, update, commit)
	if state != nil {
		slog.InfoContext(ctx, "player joined game", "playerID", player.ID, "gameID", state.ID)
	}
	return state, err
}

type MoveResult struct {
	State *model.ChessState
	Move  chess.HistMove
}

func (services *GamePlayService) NewGameMove(ctx context.Context, gameID model.GameID, player model.PlayerState, move chess.Move) (MoveResult, error) {
	update := func(state *model.ChessState) error {
		slog.InfoContext(ctx, "updating game state by making move", "player", player.ID, "gameID", gameID, "move", move)

		// pre-move validations on chess state
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
		if _, err := state.Game.NewValidMove(move); err != nil {
			return ErrInvalidMove{GameID: gameID, PlayerID: player.ID, Violation: err}
		}

		// post-move state mutations and processing
		if state.Game.Checkmate() {
			state.EndState = model.Finished
		}
		state.UndoState = model.UndoState{}
		state.Game.ClearTables()

		return nil
	}
	commit := func(pipe redis.Pipeliner, state *model.ChessState) error {
		slog.InfoContext(ctx, "committing game state when making move", "player", player.ID, "gameID", gameID)

		if !state.EndState.IsEnded() {
			return nil
		}
		if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
			return nil
		}

		result := model.WhiteWin
		if state.Game.Board.IsWhiteTurn {
			result = model.BlackWin
		}

		err := producers.ProduceFinishGame(ctx, pipe, model.FinishedGame{
			GameID:       gameID,
			WhitePlayer:  state.WhitePlayer.ID,
			BlackPlayer:  state.BlackPlayer.ID,
			ReplayMode:   state.Mode,
			ReplayResult: result,
			ReplayCause:  model.Checkmate,
		})
		return serrors.New("push finished game event", err)
	}
	state, err := services.chessState.UpdateChessStateTxn(ctx, gameID, update, commit)
	if err != nil {
		return MoveResult{}, err
	}

	histMove := state.Game.LastMove() // invariant: if this function does not error before this line, it will have at least one move.

	moveResult := MoveResult{State: state, Move: histMove}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "gameID", gameID, "moveOutput", histMove, "moveInput", move)
	return moveResult, nil
}

var (
	ErrUndoNoop       = errors.New("no undo to perform")
	ErrUndoCurrPlayer = errors.New("must not be the current player to undo a move")
	ErrNoUndo         = errors.New("no undo to accept or reject")
)

func (services *GamePlayService) AttemptGameUndo(ctx context.Context, gameID model.GameID, player model.PlayerState, kind model.UndoKind) (*model.ChessState, error) {
	update := func(state *model.ChessState) error {
		slog.InfoContext(ctx, "updating game state with undo", "gameID", gameID)

		switch kind {
		case model.UndoCreate:
			if player.IsSame(state.CurrPlayer()) {
				return ErrUndoCurrPlayer
			}
			state.UndoID = player.ID
		case model.UndoAccept:
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
		case model.UndoReject:
			if state.UndoID == 0 {
				return ErrNoUndo
			}
			state.UndoState = model.UndoState{}
		}
		return nil
	}
	state, err := services.chessState.UpdateChessStateTxn(ctx, gameID, update, nil)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "attempt game undo", "playerID", player.ID, "gameID", gameID, "gameID", state.ID)
	return state, nil
}

func (services *GamePlayService) EndGame(ctx context.Context, gameID model.GameID, player model.PlayerState) (model.EndKind, error) {
	update := func(state *model.ChessState) error {
		slog.InfoContext(ctx, "updating end game", "gameID", gameID)

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
		slog.InfoContext(ctx, "commiting end game", "gameID", gameID)

		if state.EndState != model.Finished && state.EndState != model.Aborted {
			return nil
		}
		if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
			return nil
		}

		result := model.BlackWin
		if state.BlackPlayer.ID == player.ID {
			result = model.WhiteWin
		}

		err := producers.ProduceFinishGame(ctx, pipe, model.FinishedGame{
			GameID:       gameID,
			WhitePlayer:  state.WhitePlayer.ID,
			BlackPlayer:  state.BlackPlayer.ID,
			ReplayMode:   state.Mode,
			ReplayResult: result,
			ReplayCause:  model.Forfeit,
		})
		return serrors.New("push finished game event", err)
	}
	state, err := services.chessState.UpdateChessStateTxn(ctx, gameID, update, commit)
	if err != nil {
		return model.NotEnded, err
	}

	slog.InfoContext(ctx, "player ended game", "playerID", player.ID, "gameID", gameID, "gameID", state.ID)
	return state.EndState, nil
}
