package svc

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
	"slices"
	"time"
)

func CreateGame(ctx context.Context, rdb *db.Redis, color ColorSelect, timeControl TimeControl, initialBoard *chess.Board) (string, error) {
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

	state := MakeState(StateSetup{ID: strID, TimeControl: timeControl, FirstColor: color, InitialBoard: initialBoard})
	state.Game.InitPieceMoves()

	slog.InfoContext(ctx, "created chess game", "state", state)
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
}

func JoinGame(ctx context.Context, rdb *db.Redis, gameID string, player *PlayerState) (*ChessState, error) {
	state, err := GetChessState(ctx, rdb, gameID)
	if err != nil {
		return nil, err
	}

	if player == nil {
		slog.WarnContext(ctx, "unprovided player did not join game", "state", state)
		return nil, nil
	}

	var playerExists bool
	if state.WhitePlayer == nil && state.BlackPlayer == nil {
		n, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			return nil, fmt.Errorf("generate randint used to select first color: %w", err)
		}
		pickWhite := state.FirstColor == ColorRandom && n.Int64()%2 == 0 || state.FirstColor == ColorWhite
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
		return nil, nil
	}

	err = SetChessState(ctx, rdb, gameID, state)
	util.DynLog(ctx, "player joined game", err, "playerID", player.ID, "state", state)
	return state, err
}

type MoveResult struct {
	State *ChessState
	Move  chess.HistMove
}

var (
	ErrFinishedGame = errors.New("move attempted on finished game")
	ErrTurn         = errors.New("not player's turn")
	ErrInvalidMove  = errors.New("invalid move")
)

func DoMakeMove(ctx context.Context, state *ChessState, player PlayerState, move chess.Move) (MoveResult, error) {
	var mr MoveResult

	game := &state.Game
	gameID := state.ID
	currPlayer := state.CurrPlayer()

	if state.IsEnded {
		slog.WarnContext(ctx, "make move: attempted on ended game", "gameId", gameID)
		return mr, ErrFinishedGame
	}
	if currPlayer == nil || *currPlayer != player {
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

func MakeGameMove(ctx context.Context, dbs *db.Databases, gameID string, player PlayerState, move chess.Move) (MoveResult, error) {
	var mr MoveResult

	state, err := GetChessState(ctx, dbs.Rdb, gameID)
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
		if err := WriteFinishedGame(ctx, dbs, state, result, Checkmate); err != nil {
			return mr, fmt.Errorf("write checkmate game result: %w", err)
		}
	}
	slog.InfoContext(ctx, "made move on game", "player", player.ID, "move", move, "state", state)

	if err := SetChessState(ctx, dbs.Rdb, gameID, state); err != nil {
		return mr, err
	}
	return mr, nil
}

var ErrUndoNoop = errors.New("no undo to perform")

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
		state.UndoState = UndoState{}
	}

	if err := SetChessState(ctx, rdb, gameID, state); err != nil {
		return nil, err
	}
	return state, nil
}

func ForfeitGame(ctx context.Context, dbs *db.Databases, gameID string, player PlayerState) error {
	state, err := GetChessState(ctx, dbs.Rdb, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID)
		return nil
	}

	result := BlackWin
	if state.BlackPlayer.ID == player.ID {
		result = WhiteWin
	}
	state.IsEnded = true

	if err := SetChessState(ctx, dbs.Rdb, gameID, state); err != nil {
		return err
	}
	if err := WriteFinishedGame(ctx, dbs, state, result, Forfeit); err != nil {
		return fmt.Errorf("write forfeit game result: %w", err)
	}

	slog.InfoContext(ctx, "player forfeited game", "playerID", player.ID, "gameId", gameID)
	return nil
}

func WriteFinishedGame(ctx context.Context, dbs *db.Databases, state *ChessState, result ReplayResult, cause ReplayCause) error {
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		return fmt.Errorf("room players must not be nil on a finished game: roomID: %s", state.ID)
	}
	whiteID := state.WhitePlayer.ID
	blackID := state.BlackPlayer.ID

	moveHistBytes, err := chess.MarshalMoveHistory(state.InitialBoard, state.Game.Moves)
	if err != nil {
		return fmt.Errorf("marshal move history: %w", err)
	}
	cs, err := InsertGameResultTx(ctx, dbs.Pdb, time.Time{}, GameResult{
		WhiteID:            whiteID,
		BlackID:            blackID,
		ReplayCause:        cause,
		ReplayResult:       result,
		ReplayMode:         ReplayMode(state.TimeControl), // as of right now, the replay modes only contain the time control, so we can directly cast
		SerializedMoveHist: moveHistBytes,
	})
	if err != nil {
		return fmt.Errorf("execute finish game tx: %w", err)
	}
	if cs.IsNoop() {
		return nil
	}
	slog.InfoContext(ctx, "applying elo change set to leaderboard", "changeSet", cs, "room", state.ID)

	if err := IncrLeaderboard(ctx,
		dbs.Rdb,
		UpdtLbChangeSet{ID: cs.WinID, EloDiff: cs.WinEloDiff},
		UpdtLbChangeSet{ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return fmt.Errorf("increment user leaderboard stats: %w", err)
	}
	slog.InfoContext(ctx, "finished game", "room", state.ID)
	return nil
}

type GameResult struct {
	WhiteID            int64        `json:"whiteId"`
	BlackID            int64        `json:"blackId"`
	ReplayCause        ReplayCause  `json:"cause"`
	ReplayResult       ReplayResult `json:"result"`
	ReplayMode         ReplayMode   `json:"mode"`
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
	var cs GRChangeSet
	err := pdb.RunInTx(ctx, nil,
		func(ctx context.Context, query *db.Queries) error {
			ret, err := insertGameResult(ctx, query, timeAt, params)
			cs = ret
			return err
		},
	)
	return cs, err
}

func getWhiteBlackElo(ctx context.Context, query *db.Queries, result GameResult) (whiteElo, blackElo float64, err error) {
	ids := []int64{result.WhiteID, result.BlackID}
	slices.SortFunc(ids, func(left, right int64) int { return int(left - right) }) // consistent query order

	rows, err := query.GetElosByIds(ctx, ids)
	if err != nil {
		return 0, 0, fmt.Errorf("select users %+v elo: %w", ids, err)
	}
	if len(rows) != 2 {
		return 0, 0, fmt.Errorf("expected 2 users to have elo, got %d", len(rows))
	}

	if rows[0].ID == result.WhiteID {
		whiteElo, blackElo = rows[0].Elo, rows[1].Elo
	} else {
		whiteElo, blackElo = rows[1].Elo, rows[0].Elo
	}

	return whiteElo, blackElo, nil
}

func insertGameResult(ctx context.Context, query *db.Queries, timeAt time.Time, result GameResult) (cs GRChangeSet, err error) {
	whiteElo, blackElo, err := getWhiteBlackElo(ctx, query, result)
	if err != nil {
		return cs, fmt.Errorf("get white and black elo: %w", err)
	}

	var winID, loseID int64
	var whiteEloNext, blackEloNext, winEloDiff, loseEloDiff float64

	if result.ReplayResult == Draw {
		whiteEloNext, blackEloNext = whiteElo, blackElo
	} else {
		var winElo, loseElo float64

		switch result.ReplayResult {
		case WhiteWin:
			winID, loseID, winElo, loseElo = result.WhiteID, result.BlackID, whiteElo, blackElo
		case BlackWin:
			winID, loseID, winElo, loseElo = result.BlackID, result.WhiteID, blackElo, whiteElo
		}

		winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
		loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))
		winEloDiff = winEloNext - winElo
		loseEloDiff = loseEloNext - loseElo

		switch result.ReplayResult {
		case WhiteWin:
			whiteEloNext, blackEloNext = winEloNext, loseEloNext
		case BlackWin:
			whiteEloNext, blackEloNext = loseEloNext, winEloNext
		}

		updts := []db.UpdateEloParams{{ID: winID, Elo: winEloNext, Won: true}, {ID: loseID, Elo: loseEloNext, Won: false}}
		slices.SortFunc(updts, func(left, right db.UpdateEloParams) int { return int(left.ID - right.ID) }) // consistent update order

		for _, u := range updts {
			if err := query.UpdateElo(ctx, u); err != nil {
				return cs, fmt.Errorf("update winner elo for user %d: %w", winID, err)
			}
		}
	}

	replayID, err := InsertReplay(ctx, query, ReplayInst{
		WhiteID:            result.WhiteID,
		BlackID:            result.BlackID,
		Result:             result.ReplayResult,
		Cause:              result.ReplayCause,
		Mode:               result.ReplayMode,
		WinEloDiff:         winEloDiff,
		LoseEloDiff:        loseEloDiff,
		ReplayWhiteElo:     whiteEloNext,
		ReplayBlackElo:     blackEloNext,
		SerializedMoveHist: result.SerializedMoveHist,
		PlayedOn:           timeAt,
	})
	if err != nil {
		return cs, fmt.Errorf("insert replay: %w", err)
	}

	cs = GRChangeSet{ReplayID: replayID, WinID: winID, LoseID: loseID, WinEloDiff: winEloDiff, LoseEloDiff: loseEloDiff}
	slog.InfoContext(ctx, "inserted game result", "changeSet", cs)
	return cs, nil
}
