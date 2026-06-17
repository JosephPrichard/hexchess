package svc

import (
	"context"
	"errors"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/redis/go-redis/v9"
)

func (services *HexchessServices) IsGameAccessible(ctx context.Context, id model.GameID) bool {
	gameKey := fmtGameKey(id)

	exists, err := services.redis.GameStore.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

var ErrNoChessState = errors.New("no chess state")

func (services *HexchessServices) GetChessState(ctx context.Context, id model.GameID) (*model.ChessState, error) {
	return services.getChessState(ctx, services.redis.GameStore, id)
}

type RedisChessGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func (services *HexchessServices) getChessState(ctx context.Context, getter RedisChessGetter, id model.GameID) (*model.ChessState, error) {
	gameKey := fmtGameKey(id)

	bytes, err := getter.Get(ctx, gameKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, serrors.Wrap("get chess state in redis", err)
	}

	state, err := model.UnmarshalChessState(bytes)
	if err != nil {
		return nil, serrors.Wrap("unmarshal chess state", err)
	}

	slog.InfoContext(ctx, "retrieved chess state", "key", gameKey)
	return state, nil
}

func (services *HexchessServices) SetChessState(ctx context.Context, id model.GameID, state *model.ChessState) error {
	touch := time.Now()
	return services.setChessStateAt(ctx, id, state, touch)
}

func (services *HexchessServices) setChessStateAt(ctx context.Context, id model.GameID, state *model.ChessState, touch time.Time) error {
	return services.setChessState(ctx, services.redis.GameStore, id, state, touch)
}

type RedisChessSetter interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
}

func (services *HexchessServices) setChessState(ctx context.Context, setter RedisChessSetter, id model.GameID, state *model.ChessState, updtTime time.Time) error {
	gameKey := fmtGameKey(id)

	bytes, err := proto.Marshal(model.SerializeChessState(state))
	if err != nil {
		return serrors.Wrap("marshal chess state", err)
	}
	setter.Set(ctx, gameKey, bytes, 0)

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "updtTime", updtTime)
	return nil
}

func (services *HexchessServices) setChessStates(ctx context.Context, chessStates []model.ChessState) error {
	var createdGameID []string
	pipe := services.redis.GameStore.Pipeline()

	for i := range chessStates {
		state := &chessStates[i]

		bytes, err := proto.Marshal(model.SerializeChessState(state))
		if err != nil {
			return serrors.Wrap("marshal chess state", err)
		}
		gameKey := fmtGameKey(state.ID)
		pipe.SetNX(ctx, gameKey, bytes, 0)

		createdGameID = append(createdGameID, state.ID.String())
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	slog.InfoContext(ctx, "set many chess states", "gameIDs", createdGameID)
	return nil
}

const MaxUpdateChessStateRetries = 5

var ErrMaxChessStateRetries = errors.New("update chess state txn: reached max retries")

type ChessUpdateFn func(*model.ChessState) error
type ChessCommitFn func(redis.Pipeliner, *model.ChessState) error

func (services *HexchessServices) updateChessStateTxn(ctx context.Context, gameID model.GameID, update ChessUpdateFn, commit ChessCommitFn) (*model.ChessState, error) {
	gameKey := fmtGameKey(gameID)

	for range MaxUpdateChessStateRetries {
		var ret *model.ChessState

		// standard redis Watch+Tx optimistic locking pattern to prevent the 'LostUpdate' race condition
		err := services.redis.GameStore.Watch(ctx, func(txn *redis.Tx) error {
			state, err := services.getChessState(ctx, txn, gameID)
			if err != nil {
				return err
			}

			if err := update(state); err != nil {
				return err
			}

			_, err = txn.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				err := services.setChessState(ctx, pipe, gameID, state, time.Now())
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
