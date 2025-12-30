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
	"math/big"
	"slices"
	"time"
)

func CreateGame(ctx context.Context, rdb *db.Redis, color Color, mode GameMode, initialBoard *chess.Board) (string, error) {
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

	state := MakeState(StateSetup{ID: strID, Mode: mode, FirstColor: color, InitialBoard: initialBoard})
	state.Game.InitPieceMoves()

	slog.InfoContext(ctx, "created chess game", "chessMeta", state.ChessMeta)
	if err := SetChessState(ctx, rdb, strID, &state); err != nil {
		return "", err
	}

	go broadcastGameCounts(rdb, strID)
	return strID, nil
}

func broadcastGameCounts(rdb *db.Redis, strID string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in panic while broadcasting game event", "err", r)
		}
	}()
	ctx := context.WithValue(context.Background(), logutil.Trace, "create-game-broadcast-handler")

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
}

func JoinGame(ctx context.Context, rdb *db.Redis, gameID string, player PlayerState) (*ChessState, error) {
	state, err := GetChessState(ctx, rdb, gameID)
	if err != nil {
		return nil, err
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

	err = SetChessState(ctx, rdb, gameID, state)
	logutil.DynLog(ctx, "player joined game", err, "playerID", player.ID, "chessMeta", state.ChessMeta)
	return state, err
}

type MoveResult struct {
	ReplayID int64
	State    *ChessState
	Move     chess.HistMove
}

var (
	ErrFinishedGame = errors.New("move attempted on finished game")
	ErrTurn         = errors.New("not player's turn")
	ErrInvalidMove  = errors.New("invalid move")
)

func DoMakeMove(ctx context.Context, state *ChessState, player PlayerState, move chess.Move) (MoveResult, error) {
	var mr MoveResult

	game := &state.Game
	if game.BlackMoves == nil || game.WhiteMoves == nil {
		game.InitPieceMoves()
	}

	gameID := state.ID
	currPlayer := state.CurrPlayer()

	if state.IsEnded {
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
	game.InitPieceMoves()
	state.UndoState = UndoState{}

	if game.Checkmate() {
		state.IsEnded = true
	}
	mr = MoveResult{State: state, Move: hm}
	return mr, nil
}

func MakeGameMove(ctx context.Context, databases *db.Databases, gameID string, player PlayerState, move chess.Move) (MoveResult, error) {
	var mr MoveResult

	state, err := GetChessState(ctx, databases.Rdb, gameID)
	if err != nil {
		return mr, err
	}

	mr, err = DoMakeMove(ctx, state, player, move)
	if err != nil {
		return mr, err
	}
	if state.IsEnded {
		result := WhiteWin
		if state.Game.Board.IsWhiteTurn {
			result = BlackWin
		}
		replayID, err := WriteFinishedGame(ctx, databases, state, result, Checkmate)
		if err != nil {
			return mr, fmt.Errorf("write checkmate game result: %w", err)
		}
		mr.ReplayID = replayID
	}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "chessMeta", state.ChessMeta)

	if err := SetChessState(ctx, databases.Rdb, gameID, state); err != nil {
		return mr, err
	}
	return mr, nil
}

var (
	ErrUndoNoop = errors.New("no undo to perform")
	ErrNoUndo   = errors.New("no undo to accept")
)

type UndoKind int

const (
	UndoCreate UndoKind = iota
	UndoAccept
	UndoReject
)

func AttemptGameUndo(ctx context.Context, rdb *db.Redis, gameID string, player PlayerState, kind UndoKind) (*ChessState, error) {
	state, err := GetChessState(ctx, rdb, gameID)
	if err != nil {
		return nil, err
	}

	switch kind {
	case UndoCreate:
		state.UndoID = player.ID
	case UndoAccept:
		if state.UndoID == 0 {
			slog.WarnContext(ctx, "undo: cannot accept an undo that was not created", "player", player.ID, "game", gameID)
			return nil, ErrNoUndo
		}
		if state.UndoID != player.ID {
			if err := state.UndoMove(); err != nil {
				return nil, err
			}
			state.UndoState = UndoState{}
		} else {
			slog.WarnContext(ctx, "undo: a player attempted to accept their own undo", "player", player.ID, "game", gameID)
			return nil, ErrUndoNoop
		}
	case UndoReject:
		if state.UndoID == 0 {
			slog.WarnContext(ctx, "undo: cannot reject an undo that was not created", "player", player.ID, "game", gameID)
			return nil, ErrNoUndo
		}
		state.UndoState = UndoState{}
	}

	if err := SetChessState(ctx, rdb, gameID, state); err != nil {
		return nil, err
	}
	return state, nil
}

func ForfeitGame(ctx context.Context, databases *db.Databases, gameID string, player PlayerState) (int64, error) {
	state, err := GetChessState(ctx, databases.Rdb, gameID)
	if err != nil {
		return 0, err
	}
	if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
		slog.WarnContext(ctx, "game does not have both players, cannot forfeit", "game", gameID)
		return 0, nil
	}

	result := BlackWin
	if state.BlackPlayer.ID == player.ID {
		result = WhiteWin
	}
	state.IsEnded = true

	if err := SetChessState(ctx, databases.Rdb, gameID, state); err != nil {
		return 0, err
	}
	replayID, err := WriteFinishedGame(ctx, databases, state, result, Forfeit)
	if err != nil {
		return 0, fmt.Errorf("write forfeit game result: %w", err)
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID)
	return replayID, nil
}

func WriteFinishedGame(ctx context.Context, databases *db.Databases, state *ChessState, result ReplayResult, cause ReplayCause) (int64, error) {
	if !state.WhitePlayer.Present || !state.BlackPlayer.Present {
		return 0, fmt.Errorf("room players must not be nil on a finished game: roomID: %s", state.ID)
	}
	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	moveHistBytes, err := chess.MarshalMoveHistory(state.InitialBoard, state.Game.Moves)
	if err != nil {
		return 0, fmt.Errorf("marshal move history: %w", err)
	}
	cs, err := InsertGameResultTx(ctx, databases.Pdb, time.Time{}, GameResult{
		WhiteID:            whiteID,
		BlackID:            blackID,
		ReplayCause:        cause,
		ReplayResult:       result,
		ReplayMode:         state.Mode,
		SerializedMoveHist: moveHistBytes,
	})
	if err != nil {
		return 0, fmt.Errorf("execute finish game tx: %w", err)
	}
	if cs.IsNoop() {
		return 0, nil
	}
	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", cs, "room", state.ID)

	if err := IncrLeaderboard(ctx, databases.Rdb,
		UpdtLbChangeSet{Mode: state.Mode, ID: cs.WinID, EloDiff: cs.WinEloDiff},
		UpdtLbChangeSet{Mode: state.Mode, ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return 0, fmt.Errorf("increment user leaderboard stats: %w", err)
	}
	slog.InfoContext(ctx, "completed writing finished game", "stateID", state.ID)
	return cs.ReplayID, nil
}

type GameResult struct {
	WhiteID            int64        `json:"whiteId"`
	BlackID            int64        `json:"blackId"`
	ReplayCause        ReplayCause  `json:"cause"`
	ReplayResult       ReplayResult `json:"result"`
	ReplayMode         GameMode     `json:"mode"`
	SerializedMoveHist []byte
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

func InsertGameResultTx(ctx context.Context, pdb *db.PostgreSQL, timeAt time.Time, params GameResult) (GRChangeSet, error) {
	return db.RunInTx(ctx, pdb, nil,
		func(ctx context.Context, query *db.Queries) (GRChangeSet, error) {
			return insertGameResult(ctx, query, timeAt, params)
		},
	)
}

// insertGameResult processes a game result, updates player ELO scores, and records the match details in the database.
// It handles win, loss, or draw scenarios and ensures a consistent update order for database operations to prevent deadlocking.
// Returns a GRChangeSet summarizing the changes and any error encountered during processing.
func insertGameResult(ctx context.Context, query *db.Queries, timeAt time.Time, result GameResult) (cs GRChangeSet, err error) {
	ids := []int64{result.WhiteID, result.BlackID}
	slices.SortFunc(ids, func(left, right int64) int { return int(left - right) }) // consistent query order

	mode := db.ModeEnum(result.ReplayMode.String())

	rows, err := query.SelectUserModeElosByIds(ctx, db.SelectUserModeElosByIdsParams{ID: ids, Mode: mode})
	if err != nil {
		return cs, fmt.Errorf("select users %+v elo: %w", ids, err)
	}
	whiteElo, blackElo := StartElo, StartElo
	for _, row := range rows {
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
			{UserID: result.WhiteID, Mode: mode, Draws: 1, DefaultElo: StartElo},
			{UserID: result.BlackID, Mode: mode, Draws: 1, DefaultElo: StartElo},
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

	replayID, err := InsertReplay(ctx, query, ReplayInst{
		WhiteID:            result.WhiteID,
		BlackID:            result.BlackID,
		Result:             result.ReplayResult,
		Cause:              result.ReplayCause,
		Mode:               result.ReplayMode,
		WinEloDiff:         cs.WinEloDiff,
		LoseEloDiff:        cs.LoseEloDiff,
		ReplayWhiteElo:     whiteEloNext,
		ReplayBlackElo:     blackEloNext,
		SerializedMoveHist: result.SerializedMoveHist,
		PlayedOn:           timeAt,
	})
	if err != nil {
		return cs, fmt.Errorf("insert replay: %w", err)
	}
	cs.ReplayID = replayID

	slog.InfoContext(ctx, "inserted game result", "changeSet", cs)
	return cs, nil
}
