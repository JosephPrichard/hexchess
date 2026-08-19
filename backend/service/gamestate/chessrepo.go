package gamestate

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type ChessRepoService struct {
	redis cache.Redis
}

func NewChessRepoService(redis cache.Redis) *ChessRepoService {
	return &ChessRepoService{redis: redis}
}

func (services *ChessRepoService) IsGameAccessible(ctx context.Context, id model.GameID) bool {
	defer perf.WithContext(ctx).Log()

	gameKey := cache.FmtGameKey(id)
	exists, err := services.redis.PrimaryClient.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

var ErrNoChessState = errors.New("no chess state")

func (services *ChessRepoService) GetChessState(ctx context.Context, id model.GameID) (*model.ChessState, error) {
	return services.getChessState(ctx, services.redis.PrimaryClient, id)
}

type RedisChessGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func (services *ChessRepoService) getChessState(ctx context.Context, getter RedisChessGetter, id model.GameID) (*model.ChessState, error) {
	gameKey := cache.FmtGameKey(id)

	bytes, err := getter.Get(ctx, gameKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, serrors.New("get chess state in redis", err)
	}

	state, err := model.UnmarshalChessState(bytes)
	if err != nil {
		return nil, serrors.New("unmarshal chess state", err)
	}

	slog.InfoContext(ctx, "retrieved chess state", "key", gameKey)
	return state, nil
}

func (services *ChessRepoService) SetChessState(ctx context.Context, id model.GameID, state *model.ChessState) error {
	touch := time.Now()
	return services.SetChessStatePiped(ctx, services.redis.PrimaryClient, id, state, touch)
}

type RedisChessSetter interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
}

func (services *ChessRepoService) SetChessStatePiped(ctx context.Context, setter RedisChessSetter, id model.GameID, state *model.ChessState, timer time.Time) error {
	defer perf.WithContext(ctx).Log()

	bytes, err := model.MarshalChessState(state)
	if err != nil {
		return serrors.New("marshal chess state", err)
	}

	gameKey := cache.FmtGameKey(id)
	setter.Set(ctx, gameKey, bytes, 0)

	timerScore := float64(timer.UnixMilli())
	if timerScore != 0 {
		gameTimersKey := cache.FmtGameTimersZSet(id.Partition())
		setter.ZAdd(ctx, gameTimersKey, redis.Z{Score: timerScore, Member: id.String()})
	}

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "timer", timer, "timerScore", timerScore)
	return nil
}

func (services *ChessRepoService) SetChessStates(ctx context.Context, chessStates []model.ChessState) error {
	var createdGameID []string
	pipe := services.redis.PrimaryClient.Pipeline()

	for i := range chessStates {
		state := &chessStates[i]

		bytes, err := model.MarshalChessState(state)
		if err != nil {
			return serrors.New("marshal chess state", err)
		}
		gameKey := cache.FmtGameKey(state.ID)
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

func (services *ChessRepoService) UpdateChessStateTxn(ctx context.Context, gameID model.GameID, update ChessUpdateFn, commit ChessCommitFn) (*model.ChessState, error) {
	defer perf.WithContext(ctx).Log()

	gameKey := cache.FmtGameKey(gameID)

	for i := range MaxUpdateChessStateRetries {
		var ret *model.ChessState

		// standard redis Watch+Tx optimistic locking pattern to prevent the 'LostUpdate' race condition
		err := services.redis.PrimaryClient.Watch(ctx, func(txn *redis.Tx) error {
			state, err := services.getChessState(ctx, txn, gameID)
			if err != nil {
				return err
			}

			if err := update(state); err != nil {
				return err
			}

			_, err = txn.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				err := services.SetChessStatePiped(ctx, pipe, gameID, state, time.Now())
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

		if errors.Is(err, redis.TxFailedErr) {
			slog.WarnContext(ctx, "retrying redis transaction", "error", err, "retry", i)
			continue
		}
		return ret, err
	}

	slog.ErrorContext(ctx, "exhausted redis transaction retries", "retryCount", MaxUpdateChessStateRetries)
	return nil, ErrMaxChessStateRetries
}
