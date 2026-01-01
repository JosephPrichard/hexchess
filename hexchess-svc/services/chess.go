package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/chess"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type UndoState struct {
	UndoID int64
}

type ChessState struct {
	ChessMeta
	UndoState
	InitialBoard chess.Board
	Game         chess.Game
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (s *ChessState) UndoMove() error {
	if len(s.Game.Moves) == 0 {
		return ErrNoMoveUndo
	}
	undoGame := chess.Game{Board: s.InitialBoard}
	movesExceptLast := s.Game.Moves[:len(s.Game.Moves)-1]
	for _, move := range movesExceptLast {
		undoGame.MakeMove(chess.Move{From: move.To, To: move.From, Promotion: move.Promotion})
	}
	s.Game = undoGame
	s.Game.InitPieceMoves()
	return nil
}

type ChessMeta struct {
	ID          string      `json:"id"`
	WhitePlayer PlayerState `json:"whitePlayer"`
	BlackPlayer PlayerState `json:"blackPlayer"`
	IsEnded     bool        `json:"isEnded"`
	FirstColor  Color       `json:"firstColor"`
	Mode        GameMode    `json:"mode"`
	Touch       time.Time   `json:"touch"`
}

var ChessMetaCmpOpt = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	Mode         GameMode
	FirstColor   Color
	White        *PlayerState
	Black        *PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
}

func MakeState(s StateSetup) ChessState {
	b := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		b = *s.InitialBoard
	}
	game := chess.Game{Board: b}
	if s.Game != nil {
		game = *s.Game
	}
	var whitePlayer, blackPlayer PlayerState
	if s.White != nil {
		whitePlayer = *s.White
	}
	if s.Black != nil {
		blackPlayer = *s.Black
	}
	state := ChessState{
		InitialBoard: b,
		Game:         game,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			Mode:        s.Mode,
			Touch:       time.UnixMilli(0),
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
		},
	}
	return state
}

func (s *ChessState) CurrPlayer() PlayerState {
	if s.Game.Board.IsWhiteTurn {
		return s.WhitePlayer
	}
	return s.BlackPlayer
}

func (s *ChessState) DeepCopy() ChessState {
	s2 := ChessState{
		Game:         s.Game.DeepCopy(),
		UndoState:    s.UndoState,
		InitialBoard: s.InitialBoard,
		ChessMeta: ChessMeta{
			ID:         s.ID,
			IsEnded:    s.IsEnded,
			FirstColor: s.FirstColor,
			Mode:       s.Mode,
			Touch:      s.Touch,
		},
	}
	s2.WhitePlayer = s.WhitePlayer
	s2.BlackPlayer = s.BlackPlayer
	return s2
}

func (s State) getUserGameZSet(id int64) string {
	return s.Redis.GamesZSet + "_user_" + strconv.Itoa(int(id))
}

var ErrNoChessState = errors.New("no chess state")

func (s State) GetChessState(ctx context.Context, id string) (*ChessState, error) {
	if err := s.ExpireChessStates(ctx, s.Redis.GamesZSet); err != nil {
		return nil, fmt.Errorf("expire chess states: %w", err)
	}

	fullID := "game:" + id

	data, err := s.Redis.Cache.Get(ctx, fullID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, fmt.Errorf("get chess state: %w", err)
	}

	state, err := UnmarshalChessState(data)
	if err != nil {
		return nil, fmt.Errorf("deserialize chess state: %w", err)
	}

	slog.InfoContext(ctx, "selected chess state", "key", fullID)
	return &state, nil
}

func (s State) SetChessState(ctx context.Context, id string, state *ChessState) error {
	touch := time.Now()
	return s.SetChessStateAt(ctx, id, state, touch)
}

func (s State) SetChessStateAt(ctx context.Context, id string, state *ChessState, touch time.Time) error {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := "game:" + id

	b, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}

	pipe := s.Redis.Cache.TxPipeline()
	pipe.Set(ctx, fullID, b, 0)
	pipe.ZAdd(ctx, s.Redis.GamesZSet, redis.Z{Score: touchSecs, Member: fullID})
	if state.WhitePlayer.Present {
		pipe.ZAdd(ctx, s.getUserGameZSet(state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
	}
	if state.BlackPlayer.Present {
		pipe.ZAdd(ctx, s.getUserGameZSet(state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set chess state: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return nil
}

const GameExpireFinished = 1 * time.Hour

func (s State) ExpireChessStates(ctx context.Context, zSetName string) error {
	expireBefore := time.Now().Add(-GameExpireFinished).Unix()

	keys, err := s.Redis.Cache.ZRangeByScore(ctx, zSetName, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(expireBefore, 10),
	}).Result()
	if err != nil {
		return fmt.Errorf("retrieve expired states by range: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	pipe := s.Redis.Cache.TxPipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	pipe.ZRem(ctx, zSetName, keys)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete expired states: %w", err)
	}

	slog.InfoContext(ctx, "expired chess states", "zSetName", zSetName, "keys", keys, "expireBefore", expireBefore)
	return nil
}

func (s State) GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error) {
	return s.GetChessMetas(ctx, s.getUserGameZSet(userID), 1, -1)
}

func (s State) GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error) {
	return s.GetChessMetas(ctx, s.getUserGameZSet(userID), page, count)
}

func (s State) GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error) {
	return s.GetChessMetas(ctx, s.Redis.GamesZSet, page, count)
}

func (s State) GetChessMetas(ctx context.Context, zSetName string, page, count int) ([]ChessMeta, error) {
	if page < 1 {
		page = 1
	}

	var start, stop int64
	if count >= 0 {
		start = int64((page - 1) * count)
		stop = start + int64(count) - 1
	} else {
		start = 0
		stop = -1
	}

	if err := s.ExpireChessStates(ctx, zSetName); err != nil {
		return nil, fmt.Errorf("expire chess states: %w", err)
	}

	elements, err := s.Redis.Cache.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range: %w", err)
	}
	if len(elements) == 0 {
		return nil, nil
	}

	strList, err := s.Redis.Cache.MGet(ctx, elements...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	views := make([]ChessMeta, 0, len(strList))
	for _, val := range strList {
		if val == nil {
			continue
		}
		b, ok := val.(string)
		if !ok {
			continue
		}
		cv, err := UnmarshalChessMeta([]byte(b))
		if err != nil {
			return nil, fmt.Errorf("unmarshal chess meta: %w", err)
		}
		views = append(views, cv)
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "views", views, "zSetName", zSetName, "page", page)
	return views, nil
}

func (s State) GetChessStateCount(ctx context.Context) (int64, error) {
	if err := s.ExpireChessStates(ctx, s.Redis.GamesZSet); err != nil {
		return 0, err
	}
	count, err := s.Redis.Cache.ZCard(ctx, s.Redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess states: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
