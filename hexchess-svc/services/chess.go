package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redis/go-redis/v9"
)

type UndoState struct {
	UndoID int64
}

type EndKind int

const (
	NotEnded EndKind = iota
	Aborted
	Finished
)

func (kind EndKind) isEnded() bool {
	return kind != NotEnded
}

type ChessState struct {
	ChessMeta
	UndoState
	EndState     EndKind
	InitialBoard chess.Board
	Game         chess.Game
}

func (s *ChessState) HasBothPlayers() bool {
	return s.WhitePlayer.Present && s.BlackPlayer.Present
}

func (s *ChessState) IsEitherPlayer(player PlayerState) bool {
	return s.WhitePlayer.IsSame(player) || s.BlackPlayer.IsSame(player)
}

type ChessMeta struct {
	ID          string      `json:"id"`
	WhitePlayer PlayerState `json:"whitePlayer"`
	BlackPlayer PlayerState `json:"blackPlayer"`
	FirstColor  Color       `json:"firstColor"`
	Mode        GameMode    `json:"mode"`
	Touch       time.Time   `json:"touch"`
}

var ChessMetaCmpOpt = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	Mode         GameMode
	FirstColor   Color
	White        PlayerState
	Black        PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
	EndState     EndKind
	UndoState    UndoState
}

func MakeChessState(s StateSetup) *ChessState {
	board := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		board = *s.InitialBoard
	}
	game := chess.Game{Board: board}
	if s.Game != nil {
		game = *s.Game
	}
	return &ChessState{
		InitialBoard: board,
		Game:         game,
		UndoState:    s.UndoState,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			Mode:        s.Mode,
			Touch:       time.UnixMilli(0),
			WhitePlayer: s.White,
			BlackPlayer: s.Black,
		},
		EndState: s.EndState,
	}
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (s *ChessState) Undo() error {
	if len(s.Game.Moves) == 0 {
		return ErrNoMoveUndo
	}
	index := len(s.Game.Moves) - 2 // last element minus one.
	game, err := chess.JumpMoveIndex(s.InitialBoard, s.Game.Moves, index)
	if game != nil {
		s.Game = game.DeepCopy()
	}
	return err
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
			FirstColor: s.FirstColor,
			Mode:       s.Mode,
			Touch:      s.Touch,
		},
		EndState: s.EndState,
	}
	s2.WhitePlayer = s.WhitePlayer
	s2.BlackPlayer = s.BlackPlayer
	return s2
}

func (svc *Services) IsGameAccessible(ctx context.Context, id string) bool {
	gameKey := svc.makeGameKey(id)

	exists, err := svc.Redis.GameStore.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

var ErrNoChessState = errors.New("no chess state")

func (svc *Services) GetChessState(ctx context.Context, id string) (*ChessState, error) {
	return svc.getChessStateAbstract(ctx, svc.Redis.GameStore, id)
}

type RedisGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func (svc *Services) getChessStateAbstract(ctx context.Context, getter RedisGetter, id string) (*ChessState, error) {
	gameKey := svc.makeGameKey(id)

	bytes, err := getter.Get(ctx, gameKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNoChessState
		}
		return nil, fmt.Errorf("get chess state in redis: %w", err)
	}

	state, err := UnmarshalChessState(bytes)
	if err != nil {
		return nil, fmt.Errorf("deserialize chess state: %w", err)
	}

	slog.InfoContext(ctx, "retrieved chess state", "key", gameKey)
	return &state, nil
}

func (svc *Services) SetChessState(ctx context.Context, id string, state *ChessState) error {
	touch := time.Now()
	return svc.setChessStateAt(ctx, id, state, touch)
}

func (svc *Services) setChessStateAt(ctx context.Context, id string, state *ChessState, touch time.Time) error {
	pipe := svc.Redis.GameStore.TxPipeline()
	if err := svc.setChessStatePiped(ctx, pipe, id, state, touch); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (svc *Services) setChessStatePiped(ctx context.Context, pipe redis.Pipeliner, id string, state *ChessState, touch time.Time) error {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	gameKey := svc.makeGameKey(id)

	bytes, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}
	pipe.Set(ctx, gameKey, bytes, 0)

	if state.EndState != Aborted {
		// keeps the game at the front of top of the sorted games sets on update (only stores for non guests)
		pipe.ZAdd(ctx, svc.Redis.GamesZSet, redis.Z{Score: touchSecs, Member: gameKey})
		if state.WhitePlayer.Present && !state.WhitePlayer.IsGuest {
			pipe.ZAdd(ctx, svc.getUserGameZSet(state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
		if state.BlackPlayer.Present && !state.BlackPlayer.IsGuest {
			pipe.ZAdd(ctx, svc.getUserGameZSet(state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
	} else {
		// aborted games should be removed from sorted sets, although the game itself is technically accessible
		pipe.ZRem(ctx, svc.Redis.GamesZSet, gameKey)
		pipe.ZRem(ctx, svc.getUserGameZSet(state.WhitePlayer.ID), gameKey)
		pipe.ZRem(ctx, svc.getUserGameZSet(state.BlackPlayer.ID), gameKey)
	}

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "touch", touch)
	return nil
}

const MaxUpdateChessStateRetries = 5

var ErrMaxChessStateRetries = errors.New("update chess state txn: reached max retries")

func (svc *Services) UpdateChessStateTxn(ctx context.Context, gameID string, update func(*ChessState) error) (*ChessState, error) {
	gameKey := svc.makeGameKey(gameID)

	for range MaxUpdateChessStateRetries {
		var retState *ChessState

		err := svc.Redis.GameStore.Watch(ctx, func(txn *redis.Tx) error {
			state, err := svc.getChessStateAbstract(ctx, txn, gameID)
			if err != nil {
				return err
			}

			if err := update(state); err != nil {
				return err
			}

			_, err = txn.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				return svc.setChessStatePiped(ctx, pipe, gameID, state, time.Now())
			})
			if err == nil {
				retState = state
			}
			return err
		}, gameKey)
		if err == redis.TxFailedErr {
			continue
		}
		return retState, err
	}

	return nil, ErrMaxChessStateRetries
}

func (svc *Services) GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error) {
	return svc.GetUserChessMetasPaged(ctx, userID, 1, -1)
}

func (svc *Services) GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error) {
	return svc.getChessMetas(ctx, svc.getUserGameZSet(userID), page, count)
}

func (svc *Services) GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error) {
	return svc.getChessMetas(ctx, svc.Redis.GamesZSet, page, count)
}

func (svc *Services) getChessMetas(ctx context.Context, zSetName string, page, count int) ([]ChessMeta, error) {
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

	chessKeys, err := svc.Redis.GameStore.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range: %w", err)
	}
	if len(chessKeys) == 0 {
		return nil, nil
	}

	mgetList, err := svc.Redis.GameStore.MGet(ctx, chessKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	chessViews := make([]ChessMeta, 0, len(mgetList))
	for _, val := range mgetList {
		str, ok := val.(string)
		if !ok {
			slog.Warn("chess meta mget output is not a string", "type", fmt.Sprintf("%T", val))
			continue
		}
		view, err := UnmarshalChessMeta([]byte(str))
		if err != nil {
			return nil, fmt.Errorf("unmarshal chess meta: %w", err)
		}
		chessViews = append(chessViews, view)
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "elements", chessKeys, "views", chessViews, "zSetName", zSetName, "page", page)
	return chessViews, nil
}

func (svc *Services) GetChessStateCount(ctx context.Context) (int64, error) {
	count, err := svc.Redis.GameStore.ZCard(ctx, svc.Redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
