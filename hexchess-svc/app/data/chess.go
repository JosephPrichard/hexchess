package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/app/util"
	"log/slog"
	"strconv"
	"time"
)

func getUserGameZSet(id int64) string {
	return GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb *redis.Pool, id string) (ChessState, error) {
	trace := ctx.Value(util.TraceKey)

	fail := func(str string, err error) (ChessState, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get chess state", "id", id, "err", err, "trace", trace)
		return ChessState{}, err
	}

	conn := rdb.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, GamesZSet); err != nil {
		return fail("failed to expire chess states", err)
	}
	fullID := "game:" + id

	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return ChessState{}, ErrNoChessState
	} else if err != nil {
		return fail("failed to get chess state", err)
	}

	state, err := ChessDeserialize(data)
	if err != nil {
		return fail("failed to deserialize chess state", err)
	}
	slog.Info("selected session", "key", fullID, "trace", trace)
	return state, nil
}

func SetChessState(ctx context.Context, rdb *redis.Pool, id string, state ChessState) (ChessState, error) {
	return SetChessStateAt(ctx, rdb, id, state, time.Now())
}

func SetChessStateAt(ctx context.Context, rdb *redis.Pool, id string, state ChessState, touch time.Time) (ChessState, error) {
	trace := ctx.Value(util.TraceKey)

	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := "game:" + id

	data, err := state.Serialize()
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to serialize chess state: %w", err)
	}

	conn := rdb.Get()
	defer conn.Close()

	conn.Send("MULTI")
	conn.Send("SET", fullID, data)
	conn.Send("ZADD", GamesZSet, touchSecs, fullID)
	if state.WhitePlayer != nil {
		conn.Send("ZADD", getUserGameZSet(state.WhitePlayer.ID), touchSecs, fullID)
	}
	if state.BlackPlayer != nil {
		conn.Send("ZADD", getUserGameZSet(state.BlackPlayer.ID), touchSecs, fullID)
	}
	if _, err = conn.Do("EXEC"); err != nil {
		return ChessState{}, fmt.Errorf("failed to set chess state: %w", err)
	}

	slog.Info("set chess state", "key", fullID, "touch", touch, "trace", trace)
	return state, nil
}

const GameExpireFinished = 1 * time.Hour

func ExpireChessStates(ctx context.Context, conn redis.Conn, zSetName string) error {
	return ExpireChessStatesBefore(ctx, conn, zSetName, time.Now().Add(-GameExpireFinished))
}

func ExpireChessStatesBefore(ctx context.Context, conn redis.Conn, zSetName string, expireBefore time.Time) error {
	trace := ctx.Value(util.TraceKey)

	fail := func(str string, err error) error {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to expire chess state", "zSetName", zSetName, "err", err, "trace", trace)
		return err
	}

	keys, err := redis.Values(conn.Do("ZRANGEBYSCORE", zSetName, "-inf", expireBefore.Unix()))
	if err != nil {
		return fail("failed to retrieve expired states", err)
	}
	if len(keys) == 0 {
		return nil
	}

	var delArgs []interface{}
	delArgs = append(delArgs, zSetName)
	delArgs = append(delArgs, keys...)

	conn.Send("MULTI")
	conn.Send("DEL", delArgs)
	conn.Send("ZREM", delArgs)
	if _, err = conn.Do("EXEC"); err != nil {
		return fail("failed to remove expired states", err)
	}

	slog.Info("expired chess states", "zSetName", zSetName, "keys", keys, "expireBefore", expireBefore, "err", err, "trace", trace)
	return err
}

func GetUserChessViews(ctx context.Context, rdb *redis.Pool, userID int64) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, getUserGameZSet(userID), 1, -1)
}

func GetUserChessViewsPaged(ctx context.Context, rdb *redis.Pool, userID int64, page, count int) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, getUserGameZSet(userID), page, count)
}

func GetAllChessViews(ctx context.Context, rdb *redis.Pool, page, count int) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, GamesZSet, page, count)
}

func GetChessViews(ctx context.Context, rdb *redis.Pool, zSetName string, page, count int) ([]ChessView, error) {
	trace := ctx.Value(util.TraceKey)

	fail := func(str string, err error) ([]ChessView, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get chess views", "zSetName", zSetName, "page", page, "count", count, "err", err, "trace", trace)
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	var left, right int64
	if count >= 0 {
		left = int64((page - 1) * count)
		right = left + int64(count) - 1
	} else {
		left = 0
		right = -1
	}

	conn := rdb.Get()
	defer conn.Close()

	var bytesList [][]byte

	if err := ExpireChessStates(ctx, conn, zSetName); err != nil {
		return fail("failed to expire chess states", err)
	}
	elements, err := redis.Values(conn.Do("ZREVRANGE", zSetName, left, right))
	if err != nil {
		return fail("failed to get chess id", err)
	}
	if len(elements) > 0 {
		bytesList, err = redis.ByteSlices(conn.Do("MGET", elements...))
		if err != nil {
			return fail("failed to get chess states", err)
		}
	}

	var views []ChessView
	for _, bytes := range bytesList {
		cv, err := ChessViewDeserialize(bytes)
		if err != nil {
			return fail("failed to deserialize chess view with key", err)
		}
		views = append(views, cv)
	}

	slog.Info("retrieved chess views", "count", len(views), "zSetName", zSetName, "page", page, "trace", trace)
	return views, nil
}

func GetChessStateCount(ctx context.Context, rdb *redis.Pool) (int64, error) {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, GamesZSet); err != nil {
		return 0, err
	}
	count, err := redis.Int64(conn.Do("ZCARD", GamesZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select chess count: %w", err)
	}

	slog.Info("selected chess count", "count", count, "trace", trace)
	return count, err
}
