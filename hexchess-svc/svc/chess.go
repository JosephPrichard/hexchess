package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

func getUserGameZSet(rdb *db.Redis, id int64) string {
	return rdb.GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb *db.Redis, id string) (*ChessState, error) {
	if err := ExpireChessStates(ctx, rdb, rdb.GamesZSet); err != nil {
		return nil, fmt.Errorf("expire chess states: %w", err)
	}

	fullID := "game:" + id

	data, err := rdb.Cache.Get(ctx, fullID).Bytes()
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

func SetChessState(ctx context.Context, rdb *db.Redis, id string, state *ChessState) error {
	touch := time.Now()
	return SetChessStateAt(ctx, rdb, id, state, touch)
}

func SetChessStateAt(ctx context.Context, rdb *db.Redis, id string, state *ChessState, touch time.Time) error {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := "game:" + id

	b, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return fmt.Errorf("marshal chess state: %w", err)
	}

	pipe := rdb.Cache.TxPipeline()
	pipe.Set(ctx, fullID, b, 0)
	pipe.ZAdd(ctx, rdb.GamesZSet, redis.Z{Score: touchSecs, Member: fullID})
	if state.WhitePlayer != nil {
		pipe.ZAdd(ctx, getUserGameZSet(rdb, state.WhitePlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
	}
	if state.BlackPlayer != nil {
		pipe.ZAdd(ctx, getUserGameZSet(rdb, state.BlackPlayer.ID), redis.Z{Score: touchSecs, Member: fullID})
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set chess state: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return nil
}

const GameExpireFinished = 1 * time.Hour

func ExpireChessStates(ctx context.Context, rdb *db.Redis, zSetName string) error {
	expireBefore := time.Now().Add(-GameExpireFinished).Unix()

	keys, err := rdb.Cache.ZRangeByScore(ctx, zSetName, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(expireBefore, 10),
	}).Result()
	if err != nil {
		return fmt.Errorf("retrieve expired states by range: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	pipe := rdb.Cache.TxPipeline()
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

func GetUserChessMetas(ctx context.Context, rdb *db.Redis, userID int64) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), 1, -1)
}

func GetUserChessMetasPaged(ctx context.Context, rdb *db.Redis, userID int64, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), page, count)
}

func GetAllChessMetas(ctx context.Context, rdb *db.Redis, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, rdb.GamesZSet, page, count)
}

func GetChessMetas(ctx context.Context, rdb *db.Redis, zSetName string, page, count int) ([]ChessMeta, error) {
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

	if err := ExpireChessStates(ctx, rdb, zSetName); err != nil {
		return nil, fmt.Errorf("expire chess states: %w", err)
	}

	elements, err := rdb.Cache.ZRevRange(ctx, zSetName, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("retrieve chess ids by range: %w", err)
	}
	if len(elements) == 0 {
		return nil, nil
	}

	strList, err := rdb.Cache.MGet(ctx, elements...).Result()
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

func GetChessStateCount(ctx context.Context, rdb *db.Redis) (int64, error) {
	if err := ExpireChessStates(ctx, rdb, rdb.GamesZSet); err != nil {
		return 0, err
	}
	count, err := rdb.Cache.ZCard(ctx, rdb.GamesZSet).Result()
	if err != nil {
		return 0, fmt.Errorf("count chess states: %w", err)
	}
	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
