package svc

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/redis/go-redis/v9"
)

func (services *HexchessServices) IsGameAccessible(ctx context.Context, id string) bool {
	gameKey := fmtGameKey(id)

	exists, err := services.redis.GameStore.Exists(ctx, gameKey).Result()

	return err == nil && exists == 1
}

var ErrNoChessState = errors.New("no chess state")

func (services *HexchessServices) GetChessState(ctx context.Context, id string) (*model.ChessState, error) {
	return services.getChessState(ctx, services.redis.GameStore, id)
}

type RedisChessGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

func (services *HexchessServices) getChessState(ctx context.Context, getter RedisChessGetter, id string) (*model.ChessState, error) {
	gameKey := fmtGameKey(id)

	bytes, err := getter.Get(ctx, gameKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoChessState
	} else if err != nil {
		return nil, fmt.Errorf("get chess state in redis: %w", err)
	}

	state, err := model.UnmarshalChessState(bytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal chess state: %w", err)
	}

	slog.InfoContext(ctx, "retrieved chess state", "key", gameKey)
	return state, nil
}

func (services *HexchessServices) SetChessState(ctx context.Context, id string, state *model.ChessState) error {
	touch := time.Now()
	return services.setChessStateAt(ctx, id, state, touch)
}

func (services *HexchessServices) setChessStateAt(ctx context.Context, id string, state *model.ChessState, touch time.Time) error {
	return services.setChessState(ctx, services.redis.GameStore, id, state, touch)
}

type RedisChessSetter interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
}

func (services *HexchessServices) setChessState(ctx context.Context, setter RedisChessSetter, id string, state *model.ChessState, updtTime time.Time) error {
	state.Touch = updtTime
	touchSecs := float64(state.Touch.Unix())
	gameKey := fmtGameKey(id)

	bytes, err := proto.Marshal(model.SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}
	setter.Set(ctx, gameKey, bytes, 0)

	if state.EndState != model.Aborted {
		// keeps the game at the front of top of the sorted sets on update (only stores for non guests)
		setter.ZAdd(ctx, services.redis.GamesZSet, redis.Z{Score: touchSecs, Member: gameKey})
		if state.WhitePlayer.NonGuest() {
			setter.ZAdd(ctx, fmtUserGameZSet(services.redis, state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
		if state.BlackPlayer.NonGuest() {
			setter.ZAdd(ctx, fmtUserGameZSet(services.redis, state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: gameKey})
		}
	} else {
		// aborted games should be removed from sorted sets, although the game itself is technically accessible
		setter.ZRem(ctx, services.redis.GamesZSet, gameKey)
		setter.ZRem(ctx, fmtUserGameZSet(services.redis, state.WhitePlayer.ID), gameKey)
		setter.ZRem(ctx, fmtUserGameZSet(services.redis, state.BlackPlayer.ID), gameKey)
	}

	slog.InfoContext(ctx, "set chess state", "key", gameKey, "updtTime", updtTime)
	return nil
}

func (services *HexchessServices) SetManyChessStates(ctx context.Context, chessStates []model.ChessState) error {
	var createdGameID []string
	pipe := services.redis.GameStore.Pipeline()

	for _, state := range chessStates {
		createdGameID = append(createdGameID, state.ID)
		services.setChessState(ctx, pipe, state.ID, &state, time.Now())
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	slog.InfoContext(ctx, "created many chess states", "gameIDs", createdGameID)

	return nil
}

const MaxUpdateChessStateRetries = 5

var ErrMaxChessStateRetries = errors.New("update chess state txn: reached max retries")

type ChessUpdateFn func(*model.ChessState) error
type ChessCommitFn func(redis.Pipeliner, *model.ChessState) error

func (services *HexchessServices) updateChessStateTxn(ctx context.Context, gameID string, update ChessUpdateFn, commit ChessCommitFn) (*model.ChessState, error) {
	gameKey := fmtGameKey(gameID)

	for range MaxUpdateChessStateRetries {
		var ret *model.ChessState

		// standard redis Watch+Tx optimisic locking pattern to prevent the 'LostUpdate' race condition
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

func (services *HexchessServices) getUserChessMetas(ctx context.Context, userID int64) ([]model.ChessMeta, error) {
	return services.getUserChessMetasPaged(ctx, userID, 1, -1)
}

func (services *HexchessServices) getUserChessMetasPaged(ctx context.Context, userID int64, page, count int) ([]model.ChessMeta, error) {
	return services.getChessMetas(ctx, fmtUserGameZSet(services.redis, userID), page, count)
}

func (services *HexchessServices) getAllChessMetas(ctx context.Context, page, count int) ([]model.ChessMeta, error) {
	return services.getChessMetas(ctx, services.redis.GamesZSet, page, count)
}

type ChessMetasResp struct {
	AllChessMetas  []model.ChessMeta
	SelfChessMetas []model.ChessMeta
}

func (services *HexchessServices) GetChessMetas(ctx context.Context, player enum.Optional[model.PlayerState], page int, count int) (ChessMetasResp, error) {
	var allChessMetas []model.ChessMeta
	var selfChessMetas []model.ChessMeta

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		allChessMetas, err = services.getAllChessMetas(egCtx, page, count)
		return errutil.Guardf(err, "get all chess metas page %d", page)
	})
	if player.IsPresent {
		eg.Go(func() (err error) {
			selfChessMetas, err = services.getUserChessMetas(egCtx, player.Value.ID)
			return errutil.Guardf(err, "get user %d chess metas", player.Value.ID)
		})
	}
	if err := eg.Wait(); err != nil {
		return ChessMetasResp{}, err
	}

	return ChessMetasResp{AllChessMetas: allChessMetas, SelfChessMetas: selfChessMetas}, nil
}

func (services *HexchessServices) getChessMetas(ctx context.Context, zSetName string, page, count int) ([]model.ChessMeta, error) {
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

	chessKeys, err := services.redis.GameStore.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range %d to %d: %w", start, stop, err)
	}
	if len(chessKeys) == 0 {
		return nil, nil
	}

	mGetList, err := services.redis.GameStore.MGet(ctx, chessKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get many chess states: %w", err)
	}

	chessViews := make([]model.ChessMeta, 0, len(mGetList))
	var marshalErrs []error

	for i, mGetElement := range mGetList {
		mGetString, ok := mGetElement.(string)
		if !ok {
			marshalErrs = append(marshalErrs, fmt.Errorf("chess meta %d mget output is not a string, is %T", i, mGetElement))
			continue
		}
		view, err := model.UnmarshalChessMeta([]byte(mGetString))
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

func (services *HexchessServices) GetChessStateCount(ctx context.Context) (int64, error) {
	count, err := services.redis.GameStore.ZCard(ctx, services.redis.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess state: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
