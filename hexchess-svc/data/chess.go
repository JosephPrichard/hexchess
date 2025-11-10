package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log/slog"
	"strconv"
	"time"
)

func getUserGameZSet(rdb Redis, id int64) string {
	return rdb.GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb Redis, id string) (ChessState, error) {
	fail := func(str string, err error) (ChessState, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to get chess state", "id", id, "err", err)
		return ChessState{}, err
	}

	conn := rdb.Primary.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, rdb.GamesZSet); err != nil {
		return fail("failed to expire chess states", err)
	}
	fullID := "game:" + id

	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return ChessState{}, ErrNoChessState
	} else if err != nil {
		return fail("failed to get chess state", err)
	}

	state, err := UnmarshalChess(data)
	if err != nil {
		return fail("failed to deserialize chess state", err)
	}
	slog.InfoContext(ctx, "selected chess state", "key", fullID)
	return state, nil
}

func SetChessState(ctx context.Context, rdb Redis, id string, state ChessState) (ChessState, error) {
	return SetChessStateAt(ctx, rdb, id, state, time.Now())
}

func SetChessStateAt(ctx context.Context, rdb Redis, id string, state ChessState, touch time.Time) (ChessState, error) {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := "game:" + id

	b, err := MarshalChessState(state)
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to marshal chess state: %w", err)
	}

	conn := rdb.Primary.Get()
	defer conn.Close()

	conn.Send("MULTI")
	conn.Send("SET", fullID, b)
	conn.Send("ZADD", rdb.GamesZSet, touchSecs, fullID)
	if state.WhitePlayer != nil {
		conn.Send("ZADD", getUserGameZSet(rdb, state.WhitePlayer.ID), touchSecs, fullID)
	}
	if state.BlackPlayer != nil {
		conn.Send("ZADD", getUserGameZSet(rdb, state.BlackPlayer.ID), touchSecs, fullID)
	}
	if _, err = conn.Do("EXEC"); err != nil {
		return ChessState{}, fmt.Errorf("failed to set chess state: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return state, nil
}

const GameExpireFinished = 1 * time.Hour

func ExpireChessStates(ctx context.Context, conn redis.Conn, zSetName string) error {
	return ExpireChessStatesBefore(ctx, conn, zSetName, time.Now().Add(-GameExpireFinished))
}

func ExpireChessStatesBefore(ctx context.Context, conn redis.Conn, zSetName string, expireBefore time.Time) error {
	fail := func(str string, err error) error {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to expire chess state", "zSetName", zSetName, "err", err)
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

	slog.InfoContext(ctx, "expired chess states", "zSetName", zSetName, "keys", keys, "expireBefore", expireBefore, "err", err)
	return err
}

func GetUserChessMetas(ctx context.Context, rdb Redis, userID int64) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), 1, -1)
}

func GetUserChessViewsPaged(ctx context.Context, rdb Redis, userID int64, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), page, count)
}

func GetAllChessMetas(ctx context.Context, rdb Redis, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, rdb.GamesZSet, page, count)
}

func GetChessMetas(ctx context.Context, rdb Redis, zSetName string, page, count int) ([]ChessMeta, error) {
	fail := func(str string, err error) ([]ChessMeta, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to get chess views", "zSetName", zSetName, "page", page, "count", count, "err", err)
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

	conn := rdb.Primary.Get()
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

	var metas []ChessMeta
	for _, bytes := range bytesList {
		cv, err := UnmarshalChessMeta(bytes)
		if err != nil {
			return fail("failed to unmarshal chess debug with key", err)
		}
		metas = append(metas, cv)
	}

	slog.InfoContext(ctx, "retrieved chess views", "count", len(metas), "zSetName", zSetName, "page", page)
	return metas, nil
}

func GetChessStateCount(ctx context.Context, rdb Redis) (int64, error) {

	conn := rdb.Primary.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, rdb.GamesZSet); err != nil {
		return 0, err
	}
	count, err := redis.Int64(conn.Do("ZCARD", rdb.GamesZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select chess count: %w", err)
	}

	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, err
}
