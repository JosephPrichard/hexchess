package svc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/app/chess"
	"hexchess-svc/db"
	"log/slog"
	"math/big"
	"strings"
	"time"
)

type ColorSelect int

const (
	White ColorSelect = iota
	Black
	Random
)

type TimeControl int

const (
	RealTime TimeControl = iota
	Correspondence
	Unlimited
)

type PlayerState struct {
	ID      int64
	Name    string
	Country string
	Elo     float32
	IsGuest bool
}

type ChessState struct {
	ID          string
	Game        chess.Game
	MoveList    []chess.PieceMove
	WhitePlayer *PlayerState
	BlackPlayer *PlayerState
	IsEnded     bool
	FirstColor  ColorSelect
	TimeControl TimeControl
	Touch       time.Time
}

type ChessView struct {
	ID          string
	WhitePlayer *PlayerState
	BlackPlayer *PlayerState
	IsEnded     bool
	FirstColor  ColorSelect
	TimeControl TimeControl
}

func MakeStartChessState(id string, timeControl TimeControl) ChessState {
	return ChessState{
		ID:          id,
		Game:        chess.MakeStartGame(),
		FirstColor:  Random,
		TimeControl: timeControl,
		Touch:       time.UnixMilli(0),
		//MoveList:    []chess.PieceMove{},
	}
}

func (s *ChessState) CurrPlayer() *PlayerState {
	if s.Game.Board.IsWhiteTurn {
		return s.WhitePlayer
	}
	return s.BlackPlayer
}

func ParseColorSelect(value string) (ColorSelect, error) {
	switch strings.ToUpper(value) {
	case "WHITE":
		return White, nil
	case "BLACK":
		return Black, nil
	case "RANDOM":
		return Random, nil
	default:
		return 0, fmt.Errorf("unknown color select: %s", value)
	}
}

func ParseTimeControl(value string) (TimeControl, error) {
	switch strings.ToUpper(value) {
	case "REAL_TIME":
		return RealTime, nil
	case "CORRESPONDENCE":
		return Correspondence, nil
	case "UNLIMITED":
		return Unlimited, nil
	default:
		return 0, fmt.Errorf("unknown time control: %s", value)
	}
}

type GameStoresAPI interface {
	GetChessStateCount(ctx context.Context) (int64, error)
	GetChessState(ctx context.Context, id string) (ChessState, error)
	SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error)
	UpdateGameResult(ctx context.Context, params GRParams) (GRChangeSet, error)
	UpdateLeaderboard(ctx context.Context, csList ...IncrLbChangeSet) error
}

type GameStores struct {
	Stores
}

func (a *GameStores) GetChessStateCount(ctx context.Context) (int64, error) {
	return GetChessStateCount(ctx, a.Rdb)
}

func (a *GameStores) GetChessState(ctx context.Context, id string) (ChessState, error) {
	return GetChessState(ctx, a.Rdb, id)
}

func (a *GameStores) SetChessState(ctx context.Context, id string, state ChessState) (ChessState, error) {
	return SetChessState(ctx, a.Rdb, id, state)
}

func (a *GameStores) UpdateGameResult(ctx context.Context, params GRParams) (GRChangeSet, error) {
	return UpdateGameResultTx(ctx, a.PgDB, params)
}

func (a *GameStores) UpdateLeaderboard(ctx context.Context, csList ...IncrLbChangeSet) error {
	return IncrLeaderboard(ctx, a.Rdb, csList...)
}

const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
const idLength = 8

func generateGameID() (string, error) {
	id := make([]byte, idLength)
	for i := range id {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			return "", err
		}
		id[i] = characters[n.Int64()]
	}
	return string(id), nil
}

func broadcastOnCreateGame(a GameStoresAPI) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("recovered in CreateGame broadcast handler", "err", r)
		}
	}()

	ctx := context.WithValue(context.Background(), TraceKey, "create-game-broadcast-handler")

	count, err := a.GetChessStateCount(ctx)
	if err != nil {
		slog.Error("failed to count chess states after creating game", "err", err)
		return
	}
	slog.Info("counted chess states after creating game", "count", count)

	//svc.GameBroadcaster.MultiBroker(count)
}

func CreateGame(ctx context.Context, a GameStoresAPI, color ColorSelect, timeControl TimeControl) (string, error) {
	id, err := generateGameID()
	if err != nil {
		return "", fmt.Errorf("error generating id: %w", err)
	}

	state := MakeStartChessState(id, timeControl)
	state.FirstColor = color
	state.Game.InitPieceMoves()

	slog.Info("created chess game", "state", state, "trace", ctx.Value(TraceKey))

	state, err = a.SetChessState(ctx, id, state)
	if err != nil {
		return "", err
	}

	go broadcastOnCreateGame(a)
	return id, nil
}

func JoinGame(ctx context.Context, a GameStoresAPI, gameID string, player PlayerState) (ChessState, error) {
	trace := ctx.Value(TraceKey)

	state, err := a.GetChessState(ctx, gameID)
	if err != nil {
		return ChessState{}, err
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
				return ChessState{}, err
			}
			pickWhite := state.FirstColor == Random && n.Int64()%2 == 0 || state.FirstColor == White
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

	dynLog("player joined game", err, "playerID", player.ID, "stateID", gameID, "color", color, "err", err, "trace", trace)
	return state, err
}

type MoveResult struct {
	Room ChessState
	Move chess.PieceMove
}

var (
	ErrFinishedGame = errors.New("move attempted on finished game")
	ErrTurn         = errors.New("not player's turn")
	ErrInvalidMove  = errors.New("invalid move")
)

func MakeGameMove(ctx context.Context, a GameStoresAPI, gameID string, player PlayerState, move chess.PieceMove) (MoveResult, error) {
	trace := ctx.Value(TraceKey)

	state, err := a.GetChessState(ctx, gameID)
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
		if err := handleFinishGame(ctx, a, state, isWhiteWin, Checkmate); err != nil {
			return MoveResult{}, err
		}
	}

	slog.Info("made move on game", "player", player.ID, "move", move, "state", state, "trace", trace)

	state, err = a.SetChessState(ctx, gameID, state)
	if err != nil {
		return MoveResult{}, err
	}

	return MoveResult{Room: state, Move: move}, nil
}

func handleFinishGame(ctx context.Context, a GameStoresAPI, state ChessState, isWhiteWin bool, cause ReplayCause) error {
	trace := ctx.Value(TraceKey)
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

	params := GRParams{
		WhiteID:    whiteID,
		BlackID:    blackID,
		Cause:      cause,
		IsWhiteWin: isWhiteWin,
		MoveList:   state.MoveList,
	}
	cs, err := a.UpdateGameResult(ctx, params)
	if err != nil {
		return fail("failed to execute finish game tx", err)
	}
	if cs == (GRChangeSet{}) {
		return nil
	}

	slog.Info("applying ELO change set to leaderboard", "changeSet", cs, "room", state.ID, "trace", trace)

	if err := a.UpdateLeaderboard(ctx,
		IncrLbChangeSet{ID: cs.WinID, EloDiff: cs.WinEloDiff},
		IncrLbChangeSet{ID: cs.LoseID, EloDiff: cs.LoseEloDiff},
	); err != nil {
		return fail("failed to increment user leaderboard stats", err)
	}

	slog.Info("finished game", "room", state.ID, "trace", trace)
	return nil
}

func ForfeitGame(ctx context.Context, a GameStoresAPI, gameID string, player PlayerState) error {
	trace := ctx.Value(TraceKey)

	state, err := a.GetChessState(ctx, gameID)
	if err != nil {
		return err
	}
	if state.WhitePlayer == nil || state.BlackPlayer == nil {
		slog.Warn("game does not have both players, cannot forfeit", "game", gameID, "trace", trace)
		return nil
	}

	didBlackForfeit := state.BlackPlayer.ID == player.ID
	state.IsEnded = true

	if err := handleFinishGame(ctx, a, state, didBlackForfeit, Forfeit); err != nil {
		return err
	}
	if _, err := a.SetChessState(ctx, gameID, state); err != nil {
		return err
	}

	slog.Info("player forfeited game", "playerID", player.ID, "gameId", gameID, "err", err, "trace", trace)
	return err
}

type GRParams struct {
	WhiteID    int64       `json:"whiteId"`
	BlackID    int64       `json:"blackId"`
	Cause      ReplayCause `json:"cause"`
	IsWhiteWin bool        `json:"isWhiteWin"`
	MoveList   []chess.PieceMove
}

type GRChangeSet struct {
	ReplayID    int64
	WinID       int64
	LoseID      int64
	WinEloDiff  float64
	LoseEloDiff float64
}

func UpdateGameResultTx(ctx context.Context, pgDB DB, params GRParams) (GRChangeSet, error) {
	return WithTransaction(ctx, pgDB, func(q *db.Queries) (GRChangeSet, error) {
		return UpdateGameResult(ctx, q, params)
	})
}

func UpdateGameResult(ctx context.Context, q *db.Queries, params GRParams) (GRChangeSet, error) {
	result, winID, loseID := WhiteWin, params.WhiteID, params.BlackID
	if !params.IsWhiteWin {
		result, winID, loseID = BlackWin, params.BlackID, params.WhiteID
	}

	trace := ctx.Value(TraceKey)
	fail := func(str string, err error) (GRChangeSet, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to update stats", "winID", winID, "loseID", loseID, "err", err, "trace", trace)
		return GRChangeSet{}, err
	}

	winElo, err := q.GetElo(ctx, winID)
	if err != nil {
		return fail("failed to get winner elo", err)
	}
	loseElo, err := q.GetElo(ctx, loseID)
	if err != nil {
		return fail("failed to get loser elo", err)
	}

	winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
	loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))
	winEloDiff := winEloNext - winElo
	loseEloDiff := loseEloNext - loseElo

	if winEloDiff == 0 && loseEloDiff == 0 {
		slog.Info("user stats update is a noop", "winId", winID, "loseId", loseID, "trace", trace)
		return GRChangeSet{}, nil
	}

	if err := q.UpdateWins(ctx, db.UpdateWinsParams{ID: winID, Elo: winEloNext}); err != nil {
		return fail("failed to update win elo", err)
	}
	if err := q.UpdateLosses(ctx, db.UpdateLossesParams{ID: loseID, Elo: loseEloNext}); err != nil {
		return fail("failed to update lose elo", err)
	}

	moveListJson, err := json.Marshal(params.MoveList)
	if err != nil {
		return fail("failed to unmarshal move list", err)
	}
	inst := ReplayInst{
		WhiteID:      params.WhiteID,
		BlackID:      params.BlackID,
		Result:       int32(result),
		Cause:        int32(params.Cause),
		WinElo:       winEloDiff,
		LoseElo:      loseEloDiff,
		MoveListJSON: string(moveListJson),
	}
	replayID, err := InsertReplay(ctx, q, inst)
	if err != nil {
		return fail("failed to insert replay", err)
	}

	cs := GRChangeSet{ReplayID: replayID, WinID: winID, LoseID: loseID, WinEloDiff: winEloDiff, LoseEloDiff: loseEloDiff}
	slog.Info("updated user stats", "inst", inst, "changeSet", cs, "trace", trace)
	return cs, nil
}
