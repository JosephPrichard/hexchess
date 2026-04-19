package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/redis/go-redis/v9"
)

func (svc *HexchessServices) IsGameAccessible(ctx context.Context, id string) bool {
	gameKey := svc.gameKey(id)

	exists, err := svc.redis.GameStore.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

var ErrNoChessState = errors.New("no chess state")

func (svc *HexchessServices) GetChessState(ctx context.Context, id string) (*ChessState, error) {
	return svc.getChessState(ctx, svc.redis.GameStore, id)
}

type RedisChessGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func (svc *HexchessServices) getChessState(ctx context.Context, getter RedisChessGetter, id string) (*ChessState, error) {
	gameKey := svc.gameKey(id)

	bytes, err := getter.Get(ctx, gameKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, fmt.Errorf("get chess state in redis: %w", err)
	}

	state, err := UnmarshalChessState(bytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal chess state: %w", err)
	}

	slog.InfoContext(ctx, "retrieved chess state", "key", gameKey)
	return state, nil
}

func (svc *HexchessServices) SetChessState(ctx context.Context, id string, state *ChessState) error {
	touch := time.Now()
	return svc.setChessStateAt(ctx, id, state, touch)
}

func (svc *HexchessServices) setChessStateAt(ctx context.Context, id string, state *ChessState, touch time.Time) error {
	return svc.setChessState(ctx, svc.redis.GameStore, id, state, touch)
}

type RedisChessSetter interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
}

func (svc *HexchessServices) setChessState(ctx context.Context, setter RedisChessSetter, id string, state *ChessState, updtTime time.Time) error {
	state.Touch = updtTime
	touchSecs := float64(state.Touch.Unix())
	gameKey := svc.gameKey(id)

	bytes, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}
	setter.Set(ctx, gameKey, bytes, 0)

	if state.EndState != Aborted {
		// keeps the game at the front of top of the sorted sets on update (only stores for non guests)
		setter.ZAdd(ctx, svc.redis.GamesZSet, redis.Z{Score: touchSecs, Member: gameKey})
		if state.WhitePlayer.NonGuest() {
			setter.ZAdd(ctx, svc.userGameZSet(state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
		if state.BlackPlayer.NonGuest() {
			setter.ZAdd(ctx, svc.userGameZSet(state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
	} else {
		// aborted games should be removed from sorted sets, although the game itself is technically accessible
		setter.ZRem(ctx, svc.redis.GamesZSet, gameKey)
		setter.ZRem(ctx, svc.userGameZSet(state.WhitePlayer.ID), gameKey)
		setter.ZRem(ctx, svc.userGameZSet(state.BlackPlayer.ID), gameKey)
	}

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "updtTime", updtTime)
	return nil
}

const MaxUpdateChessStateRetries = 5

var ErrMaxChessStateRetries = errors.New("update chess state txn: reached max retries")

type ChessUpdateFn func(*ChessState) error
type ChessCommitFn func(redis.Pipeliner, *ChessState) error

func (svc *HexchessServices) UpdateChessStateTxn(ctx context.Context, gameID string, update ChessUpdateFn, commit ChessCommitFn) (*ChessState, error) {
	gameKey := svc.gameKey(gameID)

	for range MaxUpdateChessStateRetries {
		var ret *ChessState

		// standard redis Watch+Tx optimisic locking pattern to prevent the 'LostUpdate' race condition
		err := svc.redis.GameStore.Watch(ctx, func(txn *redis.Tx) error {
			state, err := svc.getChessState(ctx, txn, gameID)
			if err != nil {
				return err
			}

			if err := update(state); err != nil {
				return err
			}

			_, err = txn.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				err := svc.setChessState(ctx, pipe, gameID, state, time.Now())
				if err != nil {
					return err
				}
				// enables another operation to be executed atomically if chess state update is successful
				if commit != nil {
					return commit(pipe, state)
				}
				return nil
			})
			if err == nil {
				ret = state
			}
			return err
		}, gameKey)

		if err == redis.TxFailedErr {
			continue
		}
		return ret, err
	}

	return nil, ErrMaxChessStateRetries
}

func (svc *HexchessServices) GetUserChessMetas(ctx context.Context, userID int64) ([]ChessMeta, error) {
	return svc.GetUserChessMetasPaged(ctx, userID, 1, -1)
}

func (svc *HexchessServices) GetUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]ChessMeta, error) {
	return svc.getChessMetas(ctx, svc.userGameZSet(userID), page, count)
}

func (svc *HexchessServices) GetAllChessMetas(ctx context.Context, page, count int) ([]ChessMeta, error) {
	return svc.getChessMetas(ctx, svc.redis.GamesZSet, page, count)
}

func (svc *HexchessServices) getChessMetas(ctx context.Context, zSetName string, page, count int) ([]ChessMeta, error) {
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

	chessKeys, err := svc.redis.GameStore.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range %d to %d: %w", start, stop, err)
	}
	if len(chessKeys) == 0 {
		return nil, nil
	}

	mGetList, err := svc.redis.GameStore.MGet(ctx, chessKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	chessViews := make([]ChessMeta, 0, len(mGetList))
	var marshalErrs []error

	for i, mGetElement := range mGetList {
		mGetString, ok := mGetElement.(string)
		if !ok {
			marshalErrs = append(marshalErrs, fmt.Errorf("chess meta %d mget output is not a string, is %T", i, mGetElement))
			continue
		}
		view, err := UnmarshalChessMeta([]byte(mGetString))
		if err != nil {
			marshalErrs = append(marshalErrs, fmt.Errorf("unmarshal chess meta %d: %w", i, err))
			continue
		}
		chessViews = append(chessViews, view)
	}

	if err := errors.Join(marshalErrs...); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "elements", chessKeys, "views", chessViews, "zSetName", zSetName, "page", page)
	return chessViews, nil
}

func (svc *HexchessServices) GetChessStateCount(ctx context.Context) (int64, error) {
	count, err := svc.redis.GameStore.ZCard(ctx, svc.redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
