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

func (s *State) CreateGame(ctx context.Context, color Color, mode GameMode, initialBoard *chess.Board) (string, error) {
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
	if err := s.SetChessState(ctx, strID, &state); err != nil {
		return "", fmt.Errorf("set chess state by id %s: %w", strID, err)
	}

	go s.broadcastGameCounts()
	return strID, nil
}

func (s *State) broadcastGameCounts() {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in panic while broadcasting game event", "err", r)
		}
	}()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-game-broadcast-handler")

	count, err := s.GetChessStateCount(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count chess states after creating game", "err", err)
		return
	}
	if err := s.BroadcastGameCount(ctx, count); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast chess states count after creating game", "err", err)
		return
	}
	slog.InfoContext(ctx, "counted chess states after creating game", "count", count)
}

func (s *State) JoinGame(ctx context.Context, gameID string, player PlayerState) (*ChessState, error) {
	state, err := s.GetChessState(ctx, gameID)
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

	err = s.SetChessState(ctx, gameID, state)
	if err != nil {
		return nil, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	slog.InfoContext(ctx, "player joined game", "playerID", player.ID, "chessMeta", state.ChessMeta)
	return state, nil
}

var (
	ErrFinishedGame  = errors.New("move attempted on finished game")
	ErrStartedGame   = errors.New("move attempted on not started game")
	ErrTurn          = errors.New("not player's turn")
	ErrInvalidMove   = errors.New("invalid move")
	ErrForfeitPlayer = errors.New("must be a player to forfeit or abort")
)

type MakeMoveResult struct {
	ReplayID int64
	State    *ChessState
	Move     chess.HistMove
}

func (s *State) MakeGameMove(ctx context.Context, gameID string, player PlayerState, move chess.Move) (mr MakeMoveResult, err error) {
	state, err := s.GetChessState(ctx, gameID)
	if err != nil {
		return mr, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}

	gameID = state.ID
	game := &state.Game
	if !game.HasFoundMove() {
		game.InitPieceMoves()
	}

	currPlayer := state.CurrPlayer()

	if !state.HasBothPlayers() {
		slog.WarnContext(ctx, "make move: does not have both players", "gameId", gameID)
		return mr, ErrStartedGame
	}
	if state.EndState.Kind != NotEnded {
		slog.WarnContext(ctx, "make move: attempted on ended game", "gameId", gameID)
		return mr, ErrFinishedGame
	}
	if !currPlayer.Present || currPlayer.ID != player.ID {
		slog.WarnContext(ctx, "make move: invalid turn", "player", player.ID, "game", gameID)
		return mr, ErrTurn
	}
	if err := game.ValidateMove(move); err != nil {
		slog.WarnContext(ctx, "make move: invalid move", "err", err, "player", player.ID, "move", move, "game", gameID)
		return mr, ErrInvalidMove
	}

	hm := game.MakeMove(move)
	game.AddMoveHist(hm, time.Now())
	game.InitPieceMoves()
	state.UndoState = UndoState{}

	mr = MakeMoveResult{State: state, Move: hm}

	if game.Checkmate() {
		slog.InfoContext(ctx, "game has reached checkmate", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

		result := WhiteWin
		if state.Game.Board.IsWhiteTurn {
			result = BlackWin
		}
		cause := Checkmate
		cs, err := s.WriteFinishedGame(ctx, state, result, cause)
		if err != nil {
			return mr, fmt.Errorf("write checkmate game result: %w", err)
		}
		state.EndState = EndState{
			Kind:        Finished,
			Result:      result,
			Cause:       cause,
			WinEloDiff:  int64(math.Round(cs.WinEloDiff)),
			LoseEloDiff: int64(math.Round(cs.LoseEloDiff)),
		}
	}

	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

	if err := s.SetChessState(ctx, gameID, state); err != nil {
		return mr, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
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

func (s *State) AttemptGameUndo(ctx context.Context, gameID string, player PlayerState, kind UndoKind) (*ChessState, error) {
	state, err := s.GetChessState(ctx, gameID)
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
			if err := state.Game.Undo(state.InitialBoard); err != nil {
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

	if err := s.SetChessState(ctx, gameID, state); err != nil {
		return nil, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}
	return state, nil
}

func (s *State) ForfeitGame(ctx context.Context, gameID string, player PlayerState) (replayID int64, fs EndState, err error) {
	state, err := s.GetChessState(ctx, gameID)
	if err != nil {
		return replayID, fs, fmt.Errorf("get chess state by id %s: %w", gameID, err)
	}
	if state.EndState.Kind != NotEnded {
		slog.WarnContext(ctx, "forfeit game: attempted on ended game", "gameId", gameID)
		return replayID, fs, ErrFinishedGame
	}
	if !state.IsEitherPlayer(player) {
		return replayID, fs, ErrForfeitPlayer
	}

	isForfeit := state.WhitePlayer.Present && state.BlackPlayer.Present

	if isForfeit {
		result := BlackWin
		if state.BlackPlayer.ID == player.ID {
			result = WhiteWin
		}
		cause := Forfeit

		cs, err := s.WriteFinishedGame(ctx, state, result, cause)
		if err != nil {
			return replayID, fs, fmt.Errorf("write forfeit game result: %w", err)
		}
		replayID = cs.ReplayID

		fs = EndState{
			Kind:        Finished,
			Result:      result,
			Cause:       cause,
			WinEloDiff:  int64(math.Round(cs.WinEloDiff)),
			LoseEloDiff: int64(math.Round(cs.LoseEloDiff)),
		}
	} else {
		fs = EndState{Kind: Aborted}
	}

	state.EndState = fs
	if err := s.SetChessState(ctx, gameID, state); err != nil {
		return replayID, fs, fmt.Errorf("set chess state by id %s: %w", gameID, err)
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID)
	return replayID, fs, nil
}

func (s *State) WriteFinishedGame(ctx context.Context, state *ChessState, result ReplayResult, cause ReplayCause) (cs GRChangeSet, err error) {
	if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
		return cs, fmt.Errorf("game players must be present on a finished game: %s", state.ID)
	}
	if state.WhitePlayer.ID < 0 || state.BlackPlayer.ID < 0 {
		slog.WarnContext(ctx, "one or more players for game is a guest, did not write finished game", "gameID", state.ID, "white", state.WhitePlayer, "black", state.BlackPlayer)
		return cs, nil
	}

	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	cs, err = s.InsertGameResultTx(ctx, time.Time{}, GameResult{
		WhiteID:      whiteID,
		BlackID:      blackID,
		ReplayCause:  cause,
		ReplayResult: result,
		ReplayMode:   state.Mode,
	})
	if err != nil {
		return cs, fmt.Errorf("execute finish game tx: %w", err)
	}
	if cs.IsNoop() {
		return cs, nil
	}
	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", cs, "room", state.ID)

	go func() {
		if err := s.PutReplayMoveSeq(ctx, cs.ReplayID, state.InitialBoard, state.Game.Moves); err != nil {
			// it is high unlikely that this fails, but consider adding a DLQ here
			slog.ErrorContext(ctx, "failed upload replay move seq to S3", "err", err)
		}
	}()
	go func() {
		if err := s.IncrLeaderboard(ctx,
			UpdtLbChangeSet{Mode: state.Mode, ID: cs.WinID, EloDiff: cs.WinEloDiff},
			UpdtLbChangeSet{Mode: state.Mode, ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
		); err != nil {
			// we can just log if this operation fails, the leaderboard will get sync'd eventually with the sync leaderboard job.
			slog.ErrorContext(ctx, "failed to incr leaderboard", "err", err, "changeSet", cs)
		}
	}()

	slog.InfoContext(ctx, "completed writing finished game", "stateID", state.ID)
	return cs, nil
}

type GameResult struct {
	WhiteID      int64        `json:"whiteId"`
	BlackID      int64        `json:"blackId"`
	ReplayCause  ReplayCause  `json:"cause"`
	ReplayResult ReplayResult `json:"result"`
	ReplayMode   GameMode     `json:"mode"`
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

func (s *State) InsertGameResultTx(ctx context.Context, timeAt time.Time, params GameResult) (cs GRChangeSet, err error) {
	err = s.RunInTx(ctx, db.TxnArgs{
		Fn: func(ctx context.Context, query *db.Queries) (err error) {
			cs, err = insertGameResult(ctx, query, timeAt, params)
			return err
		},
	})
	return cs, err
}

// insertGameResult processes a game result, updates player ELO scores, and records the match details in the database.
// It handles win, loss, or draw scenarios and ensures a consistent update order for database operations to prevent deadlocking.
// Returns a GRChangeSet summarizing the changes and any error encountered during processing.
func insertGameResult(ctx context.Context, query *db.Queries, timeAt time.Time, result GameResult) (cs GRChangeSet, err error) {
	ids := []int64{result.WhiteID, result.BlackID}
	slices.SortFunc(ids, func(left, right int64) int { return int(left - right) }) // consistent query order

	mode := db.ModeEnum(result.ReplayMode.String())

	rowElos, err := query.SelectUserModeElosByIds(ctx, db.SelectUserModeElosByIdsParams{ID: ids, Mode: mode})
	if err != nil {
		return cs, fmt.Errorf("select users %+v elo: %w", ids, err)
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
				return cs, fmt.Errorf("upserting draw elo for user %d: %w", updt.UserID, err)
			}
		}
	} else {
		var winElo, loseElo float64

		switch result.ReplayResult {
		case WhiteWin:
			cs.WinID, cs.LoseID, winElo, loseElo = result.WhiteID, result.BlackID, whiteElo, blackElo
		case BlackWin:
			cs.WinID, cs.LoseID, winElo, loseElo = result.BlackID, result.WhiteID, blackElo, whiteElo
		default:
		}

		winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))
		cs.WinEloDiff = winEloNext - winElo
		cs.LoseEloDiff = loseEloNext - loseElo

		switch result.ReplayResult {
		case WhiteWin:
			whiteEloNext, blackEloNext = winEloNext, loseEloNext
		case BlackWin:
			whiteEloNext, blackEloNext = loseEloNext, winEloNext
		default:
		}

		updts := []db.UpsertEloParams{
			{UserID: cs.WinID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: winEloNext}, Wins: 1, DefaultElo: StartElo},
			{UserID: cs.LoseID, Mode: mode, Elo: pgtype.Float8{Valid: true, Float64: loseEloNext}, Losses: 1, DefaultElo: StartElo},
		}
		slices.SortFunc(updts, func(left, right db.UpsertEloParams) int { return int(left.UserID - right.UserID) }) // consistent update order

		for _, updt := range updts {
			if err := query.UpsertElo(ctx, updt); err != nil {
				return cs, fmt.Errorf("upserting win/loss elo for user %d: %w", updt.UserID, err)
			}
		}
	}

	replayID, err := insertReplay(ctx, query, ReplayInst{
		WhiteID:        result.WhiteID,
		BlackID:        result.BlackID,
		Result:         result.ReplayResult,
		Cause:          result.ReplayCause,
		Mode:           result.ReplayMode,
		WinEloDiff:     cs.WinEloDiff,
		LoseEloDiff:    cs.LoseEloDiff,
		ReplayWhiteElo: whiteEloNext,
		ReplayBlackElo: blackEloNext,
		PlayedOn:       timeAt,
	})
	if err != nil {
		return cs, fmt.Errorf("insert replay: %w", err)
	}
	cs.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", cs)
	return cs, nil
}
