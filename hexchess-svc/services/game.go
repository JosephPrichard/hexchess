package svc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"log/slog"
	"math"
	"math/big"
	"slices"
	"time"
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
	if err := svc.SetChessState(ctx, strID, &state); err != nil {
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

	err = svc.SetChessState(ctx, gameID, state)
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
	Kind   EndKind
}

func (e ErrFinishedGame) Error() string {
	return fmt.Sprintf("attempted on ended game (gameId=%s, kind=%v)", e.GameID, e.Kind)
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
	var moveResult MakeMoveResult

	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return moveResult, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	gameID = state.ID
	state.Game.EnsurePieceMoves()

	if !state.HasBothPlayers() {
		return moveResult, ErrStartedGame{GameID: gameID}
	}
	if state.EndState.Kind != NotEnded {
		return moveResult, ErrFinishedGame{GameID: gameID, Kind: state.EndState.Kind}
	}
	currPlayer := state.CurrPlayer()
	if !currPlayer.Present || currPlayer.ID != player.ID {
		return moveResult, ErrTurn{GameID: gameID, PlayerID: player.ID, CurrID: currPlayer.ID}
	}
	histMove, err := state.Game.MakeValidMove(move)
	if err != nil {
		return moveResult, ErrInvalidMove{GameID: gameID, PlayerID: player.ID, Violation: err}
	}
	isCheckmate := state.Game.Checkmate()
	state.UndoState = UndoState{}

	// this lock is used to guarantee one client may write the game state at any given time. a client will retry the operation (and read again) if it fails to acquire a lock, so we can acquire after reading.
	if ok := svc.AcquireChessLock(ctx, gameID); !ok {
		return moveResult, ErrLockedGame
	}
	defer svc.ReleaseChessLock(ctx, gameID) // if this fails the lock will expire anyways, so don't handle the error.

	// write the results of the operation to all stores. if the game is complete, we must persist game stats information to the databases. this operation is not atomic.
	if isCheckmate {
		slog.InfoContext(ctx, "game has reached checkmate", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

		result := WhiteWin
		if state.Game.Board.IsWhiteTurn {
			result = BlackWin
		}
		cause := Checkmate
		nextEndState, err := svc.InsertFinishedGame(ctx, state, result, cause)
		if err != nil {
			return moveResult, fmt.Errorf("write checkmate game result: %w", err)
		}
		state.EndState = nextEndState // end state is double written to redis
	}
	if err := svc.SetChessState(ctx, gameID, state); err != nil {
		return moveResult, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}

	moveResult = MakeMoveResult{State: state, Move: histMove}
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

	if err := svc.SetChessState(ctx, gameID, state); err != nil {
		return nil, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	return state, nil
}

func (svc *Services) ForfeitGame(ctx context.Context, gameID string, player PlayerState) (endState EndState, err error) {
	state, err := svc.GetChessState(ctx, gameID)
	if err != nil {
		return endState, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	if state.EndState.Kind != NotEnded {
		return endState, ErrFinishedGame{GameID: gameID, Kind: state.EndState.Kind}
	}
	if !state.IsEitherPlayer(player) {
		return endState, ErrForfeitPlayer
	}

	isForfeit := state.WhitePlayer.Present && state.BlackPlayer.Present
	if isForfeit {
		result := BlackWin
		if state.BlackPlayer.ID == player.ID {
			result = WhiteWin
		}
		cause := Forfeit

		nextEndState, err := svc.InsertFinishedGame(ctx, state, result, cause)
		if err != nil {
			return endState, fmt.Errorf("write forfeit game result: %w", err)
		}
		endState = nextEndState
	} else {
		endState = EndState{Kind: Aborted}
	}
	state.EndState = endState // end state is double written to redis

	if err := svc.SetChessState(ctx, gameID, state); err != nil {
		return endState, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID, "endState", endState)
	return endState, nil
}

func (svc *Services) InsertFinishedGame(ctx context.Context, state *ChessState, result ReplayResult, cause ReplayCause) (nextEndState EndState, err error) {
	if state == nil {
		return nextEndState, errors.New("precondition violation: state must not be nil")
	}
	var changeSet GRChangeSet

	if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
		return nextEndState, fmt.Errorf("game players must be present on a finished game: %s", state.ID)
	}
	if state.WhitePlayer.ID < 0 || state.BlackPlayer.ID < 0 {
		slog.WarnContext(ctx, "one or more players for game is a guest, did not write finished game", "gameID", state.ID, "white", state.WhitePlayer, "black", state.BlackPlayer)
		return nextEndState, nil
	}
	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	moveHistBlob, err := chess.MarshalMoveHistory(state.InitialBoard, state.Game.Moves)
	if err != nil {
		return nextEndState, fmt.Errorf("marshal move history to s3: %w", err)
	}

	changeSet, err = svc.InsertGameResultTx(ctx, time.Time{}, GameResult{
		GameID:       state.ID,
		WhiteID:      whiteID,
		BlackID:      blackID,
		ReplayCause:  cause,
		ReplayResult: result,
		ReplayMode:   state.Mode,
		MoveHistBlob: moveHistBlob,
	})
	if err != nil {
		return nextEndState, fmt.Errorf("insert finish game tx: %w", err)
	}

	if !changeSet.IsNoop() {
		slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", changeSet, "room", state.ID)

		go func() {
			if err := svc.IncrLeaderboard(ctx,
				UpdtLbChangeSet{Mode: state.Mode, ID: changeSet.WinID, EloDiff: changeSet.WinEloDiff},
				UpdtLbChangeSet{Mode: state.Mode, ID: changeSet.LoseID, EloDiff: changeSet.LoseEloDiff},
			); err != nil {
				// we can just log if this operation fails, the leaderboard will get sync'd eventually with the sync leaderboard job.
				slog.ErrorContext(ctx, "failed to incr leaderboard", "err", err, "changeSet", changeSet)
			}
		}()
	}

	nextEndState = EndState{
		ReplayID:    changeSet.ReplayID,
		Kind:        Finished,
		Result:      result,
		Cause:       cause,
		WinEloDiff:  int64(math.Round(changeSet.WinEloDiff)),
		LoseEloDiff: int64(math.Round(changeSet.LoseEloDiff)),
	}
	slog.InfoContext(ctx, "completed writing finished game", "ID", state.ID, "nextEndState", nextEndState)
	return nextEndState, nil
}

type GameResult struct {
	GameID       string       `json:"gameId"`
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	ReplayCause  ReplayCause  `json:"cause"`
	ReplayResult ReplayResult `json:"result"`
	ReplayMode   GameMode     `json:"mode"`
	MoveHistBlob []byte
}

type GRChangeSet struct {
	ReplayID    int64
	WinID       int64
	LoseID      int64
	WinEloDiff  float64
	LoseEloDiff float64
}

func (cs GRChangeSet) IsNoop() bool {
	return cs.LoseEloDiff == 0 && cs.WinEloDiff == 0
}

// InsertGameResultTx executes the insertGameResult operation in a transaction primarily to ensure
func (svc *Services) InsertGameResultTx(ctx context.Context, timeAt time.Time, params GameResult) (cs GRChangeSet, err error) {
	err = svc.RunInTx(ctx, db.TxnArgs{
		QueryFn: func(ctx context.Context, query *db.Queries) (err error) {
			cs, err = insertGameResult(ctx, query, timeAt, params)
			return err
		},
	})
	return cs, err
}

// insertGameResult processes a game result, updates player ELO scores, and records the match details in the database.
// It handles win, loss, or draw scenarios and ensures a consistent update order for database operations to prevent deadlocking.
// Returns a GRChangeSet summarizing the changes and any error encountered during processing.
func insertGameResult(ctx context.Context, query *db.Queries, timeAt time.Time, result GameResult) (changeSet GRChangeSet, err error) {

	ids := []int64{result.WhiteID, result.BlackID}
	slices.SortFunc(ids, func(left, right int64) int { return int(left - right) }) // consistent query order

	mode := db.ModeEnum(result.ReplayMode.String())

	rowElos, err := query.SelectUserModeElosByIds(ctx, db.SelectUserModeElosByIdsParams{ID: ids, Mode: mode})
	if err != nil {
		return changeSet, fmt.Errorf("select users %+v elo: %w", ids, err)
	}

	whiteElo, blackElo := StartElo, StartElo
	for _, row := range rowElos {
		switch row.UserID {
		case result.WhiteID:
			whiteElo = row.Elo
		case result.BlackID:
			blackElo = row.Elo
		}
	}

	var whiteEloNext, blackEloNext float64

	if result.ReplayResult == Draw {
		whiteEloNext, blackEloNext = whiteElo, blackElo

		updts := []db.UpsertEloParams{
			{UserID: result.WhiteID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: whiteEloNext}, Draws: 1, DefaultElo: StartElo},
			{UserID: result.BlackID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: blackEloNext}, Draws: 1, DefaultElo: StartElo},
		}
		slices.SortFunc(updts, func(left, right db.UpsertEloParams) int { return int(left.UserID - right.UserID) }) // consistent update order

		for _, updt := range updts {
			if err := query.UpsertElo(ctx, updt); err != nil {
				return changeSet, fmt.Errorf("upserting draw elo for user %d: %w", updt.UserID, err)
			}
		}
	} else {
		var winElo, loseElo float64

		switch result.ReplayResult {
		case WhiteWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.WhiteID, result.BlackID, whiteElo, blackElo
		case BlackWin:
			changeSet.WinID, changeSet.LoseID, winElo, loseElo = result.BlackID, result.WhiteID, blackElo, whiteElo
		default:
		}

		winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))
		changeSet.WinEloDiff = winEloNext - winElo
		changeSet.LoseEloDiff = loseEloNext - loseElo

		switch result.ReplayResult {
		case WhiteWin:
			whiteEloNext, blackEloNext = winEloNext, loseEloNext
		case BlackWin:
			whiteEloNext, blackEloNext = loseEloNext, winEloNext
		default:
		}

		updts := []db.UpsertEloParams{
			{UserID: changeSet.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: StartElo},
			{UserID: changeSet.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: StartElo},
		}
		slices.SortFunc(updts, func(left, right db.UpsertEloParams) int { return int(left.UserID - right.UserID) }) // consistent update order

		for _, updt := range updts {
			if err := query.UpsertElo(ctx, updt); err != nil {
				return changeSet, fmt.Errorf("upserting win/loss elo for user %d: %w", updt.UserID, err)
			}
		}
	}

	replayID, err := insertReplay(ctx, query, ReplayInst{
		WhiteID:        result.WhiteID,
		BlackID:        result.BlackID,
		Result:         result.ReplayResult,
		Cause:          result.ReplayCause,
		Mode:           result.ReplayMode,
		WinEloDiff:     changeSet.WinEloDiff,
		LoseEloDiff:    changeSet.LoseEloDiff,
		ReplayWhiteElo: whiteEloNext,
		ReplayBlackElo: blackEloNext,
		PlayedOn:       timeAt,
		MoveHistBlob:   result.MoveHistBlob,
	})
	if err != nil {
		return changeSet, fmt.Errorf("insert replay: %w", err)
	}
	changeSet.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", changeSet)
	return changeSet, nil
}
