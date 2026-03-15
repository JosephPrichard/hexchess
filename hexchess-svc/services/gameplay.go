package svc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/pkg/logutil"
	"log/slog"
	"math/big"
)

func (svc *Services) CreateGame(ctx context.Context, color Color, mode GameMode, initialBoard *chess.Board) (string, error) {
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

	state := MakeChess(StateSetup{ID: strID, Mode: mode, FirstColor: color, InitialBoard: initialBoard})
	state.Game.InitPieceMoves()

	slog.InfoContext(ctx, "created chess game", "chessMeta", state.ChessMeta)
	if err := svc.SetChessStateNow(ctx, strID, &state); err != nil {
		return "", fmt.Errorf("set chess state by id %s: %w", strID, err)
	}

	go func() {
		if err := svc.broadcastGameCounts(); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast game count after creating game", "err", err)
		}
	}()
	return strID, nil
}

func (svc *Services) broadcastGameCounts() error {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in panic while broadcasting game event", "err", r)
		}
	}()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-game-broadcast-handler")

	count, err := svc.GetChessStateCount(ctx)
	if err != nil {
		return fmt.Errorf("failed to count chess game after creating game: %w", err)
	}
	if err := svc.BroadcastGameCount(ctx, count); err != nil {
		return fmt.Errorf("failed to broadcast chess game count after creating game: %w", err)
	}
	slog.InfoContext(ctx, "counted games after creating game", "count", count)
	return nil
}

func (svc *Services) JoinGame(ctx context.Context, gameID string, player PlayerState) (*ChessState, error) {
	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	if !player.Present {
		slog.WarnContext(ctx, "player did not join the game", "playerID", player.ID)
		return state, nil
	}

	var playerExists bool
	if !state.WhitePlayer.Present && !state.BlackPlayer.Present {
		n, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			return nil, fmt.Errorf("generate randint used to select first color: %w", err)
		}
		pickWhite := state.FirstColor == Random && n.Int64()%2 == 0 || state.FirstColor == White
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
		return state, nil
	}

	err = svc.SetChessStateNow(ctx, gameID, state)
	if err != nil {
		return nil, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	slog.InfoContext(ctx, "player joined game", "playerID", player.ID, "chessMeta", state.ChessMeta)
	return state, nil
}

var (
	ErrLockedGame    = errors.New("game is locked")
	ErrForfeitPlayer = errors.New("must be a player to forfeit or abort")
)

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

type MakeMoveResult struct {
	ReplayID int64
	State    *ChessState
	Move     chess.HistMove
}

func (svc *Services) MakeGameMove(ctx context.Context, gameID string, player PlayerState, move chess.Move) (MakeMoveResult, error) {
	var mr MakeMoveResult

	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return mr, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	gameID = state.ID
	state.Game.EnsurePieceMoves()

	if !state.HasBothPlayers() {
		return mr, ErrStartedGame{GameID: gameID}
	}
	if state.EndState.isEnded() {
		return mr, ErrFinishedGame{GameID: gameID}
	}
	currPlayer := state.CurrPlayer()
	if !currPlayer.Present || currPlayer.ID != player.ID {
		return mr, ErrTurn{GameID: gameID, PlayerID: player.ID, CurrID: currPlayer.ID}
	}
	histMove, err := state.Game.MakeValidMove(move)
	if err != nil {
		return mr, ErrInvalidMove{GameID: gameID, PlayerID: player.ID, Violation: err}
	}
	isCheckmate := state.Game.Checkmate()
	state.UndoState = UndoState{}

	// this lock is used to guarantee one client may write the game state at any given time. a client will retry the operation (and read again) if it fails to acquire a lock, so we can acquire after reading.
	if ok := svc.AcquireChessLock(ctx, gameID); !ok {
		return mr, ErrLockedGame
	}
	defer svc.ReleaseChessLock(ctx, gameID)

	if isCheckmate {
		state.EndState = Finished
	}
	if err := svc.SetChessStateNow(ctx, gameID, state); err != nil {
		return mr, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}

	// write the results of the operation to all stores. if the game is complete, we must persist game stats information to the databases. this operation is not atomic.
	if state.EndState.isEnded() {
		slog.InfoContext(ctx, "game has reached checkmate", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

		result := WhiteWin
		if state.Game.Board.IsWhiteTurn {
			result = BlackWin
		}
		if err := svc.PushFinishGameEvent(ctx, FinishGameEvent{
			GameID:       gameID,
			WhitePlayer:  state.WhitePlayer,
			BlackPlayer:  state.BlackPlayer,
			ReplayMode:   state.Mode,
			ReplayResult: result,
			ReplayCause:  Checkmate,
		}); err != nil {
			return mr, fmt.Errorf("push finished game event: %w", err)
		}
	}

	mr = MakeMoveResult{State: state, Move: histMove}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "mr", mr, "move", move, "chessMeta", state.ChessMeta)
	return mr, nil
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

func (svc *Services) AttemptGameUndo(ctx context.Context, gameID string, player PlayerState, kind UndoKind) (*ChessState, error) {
	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}

	switch kind {
	case UndoCreate:
		if player.IsSame(state.CurrPlayer()) {
			return nil, ErrUndoCurrPlayer
		}
		state.UndoID = player.ID
	case UndoAccept:
		if state.UndoID == 0 {
			return nil, ErrNoUndo
		}
		if state.UndoID != player.ID {
			if err := state.Undo(); err != nil {
				return nil, err
			}
			state.UndoState = UndoState{}
		} else {
			return nil, ErrUndoNoop
		}
	case UndoReject:
		if state.UndoID == 0 {
			return nil, ErrNoUndo
		}
		state.UndoState = UndoState{}
	}

	if err := svc.SetChessStateNow(ctx, gameID, state); err != nil {
		return nil, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	return state, nil
}

func (svc *Services) EndGame(ctx context.Context, gameID string, player PlayerState) error {
	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	if state.EndState.isEnded() {
		return ErrFinishedGame{GameID: gameID}
	}
	if !state.IsEitherPlayer(player) {
		return ErrForfeitPlayer
	}
	if state.EndState.isEnded() {
		return ErrFinishedGame{GameID: gameID}
	}

	isForfeit := state.WhitePlayer.Present && state.BlackPlayer.Present

	if isForfeit {
		state.EndState = Finished
	} else {
		state.EndState = Aborted
	}
	if err := svc.SetChessStateNow(ctx, gameID, state); err != nil {
		return fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}

	if isForfeit {
		result := BlackWin
		if state.BlackPlayer.ID == player.ID {
			result = WhiteWin
		}

		if err := svc.PushFinishGameEvent(ctx, FinishGameEvent{
			GameID:       gameID,
			WhitePlayer:  state.WhitePlayer,
			BlackPlayer:  state.BlackPlayer,
			ReplayMode:   state.Mode,
			ReplayResult: result,
			ReplayCause:  Forfeit,
		}); err != nil {
			return fmt.Errorf("push finished game event: %w", err)
		}
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID)
	return nil
}
