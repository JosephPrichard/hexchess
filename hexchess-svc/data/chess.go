package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
)

func getUserGameZSet(rdb *Redis, id int64) string {
	return rdb.GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb *Redis, id string) (ChessState, error) {
	var state ChessState

	conn := rdb.Cache.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, rdb.GamesZSet); err != nil {
		return state, fmt.Errorf("failed to expire chess states: %w", err)
	}

	fullID := "game:" + id

	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return ChessState{}, ErrNoChessState
	} else if err != nil {
		return state, fmt.Errorf("failed to 'GET' chess state: %w", err)
	}

	state, err = UnmarshalChessState(data)
	if err != nil {
		return state, fmt.Errorf("failed to deserialize chess state: %w", err)
	}

	slog.InfoContext(ctx, "selected chess state", "key", fullID)
	return state, nil
}

func SetChessState(ctx context.Context, rdb *Redis, id string, state ChessState) (ChessState, error) {
	touch := time.Now()
	return SetChessStateAt(ctx, rdb, id, state, touch)
}

func SetChessStateAt(ctx context.Context, rdb *Redis, id string, state ChessState, touch time.Time) (ChessState, error) {
	state.Touch = touch
	touchSecs := float64(state.Touch.Unix())
	fullID := "game:" + id

	b, err := proto.Marshal(SerializeChessState(state))
	if err != nil {
		return state, fmt.Errorf("failed to marshal chess state: %w", err)
	}

	conn := rdb.Cache.Get()
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
		return state, fmt.Errorf("failed to 'SET' chess state: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return state, nil
}

const GameExpireFinished = 1 * time.Hour

func ExpireChessStates(ctx context.Context, conn redis.Conn, zSetName string) error {
	fail := func(err error) error {
		slog.ErrorContext(ctx, "failed to expire chess states", "err", err, "zSetName", zSetName)
		return err
	}

	expireBefore := time.Now().Add(-GameExpireFinished)

	keys, err := redis.Values(conn.Do("ZRANGEBYSCORE", zSetName, "-inf", expireBefore.Unix()))
	if err != nil {
		return fail(fmt.Errorf("failed to 'ZRANGEBYSCORE' expired states: %w", err))
	}
	if len(keys) == 0 {
		return nil
	}

	delArgs := keys
	zRemArgs := append([]interface{}{zSetName}, keys...)

	conn.Send("MULTI")
	conn.Send("DEL", delArgs...)
	conn.Send("ZREM", zRemArgs...)
	if _, err = conn.Do("EXEC"); err != nil {
		return fail(fmt.Errorf("failed to 'DEL' expired states: %w", err))
	}

	keyStrs := make([]string, 0, len(keys))
	for _, key := range keys {
		keyStrs = append(keyStrs, fmt.Sprintf("%s", key))
	}

	slog.InfoContext(ctx, "expired chess states", "zSetName", zSetName, "keys", keyStrs, "expireBefore", expireBefore)
	return nil
}

func GetUserChessMetas(ctx context.Context, rdb *Redis, userID int64) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), 1, -1)
}

func GetUserChessMetasPaged(ctx context.Context, rdb *Redis, userID int64, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, getUserGameZSet(rdb, userID), page, count)
}

func GetAllChessMetas(ctx context.Context, rdb *Redis, page, count int) ([]ChessMeta, error) {
	return GetChessMetas(ctx, rdb, rdb.GamesZSet, page, count)
}

func GetChessMetas(ctx context.Context, rdb *Redis, zSetName string, page, count int) ([]ChessMeta, error) {
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

	fail := func(err error) ([]ChessMeta, error) {
		slog.ErrorContext(ctx, "failed to get chess views", "err", err, "zSetName", zSetName, "page", page, "count", count)
		return nil, err
	}

	conn := rdb.Cache.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, zSetName); err != nil {
		return fail(fmt.Errorf("failed to expire chess states: %w", err))
	}

	elements, err := redis.Values(conn.Do("ZREVRANGE", zSetName, left, right))
	if err != nil {
		return fail(fmt.Errorf("failed to 'ZREVRANGE' chess id: %w", err))
	}

	var bytesList [][]byte
	if len(elements) > 0 {
		bytesList, err = redis.ByteSlices(conn.Do("MGET", elements...))
		if err != nil {
			return fail(fmt.Errorf("failed to 'MGET' chess states: %w", err))
		}
	}

	views := make([]ChessMeta, 0)
	for _, bytes := range bytesList {
		cv, err := UnmarshalChessMeta(bytes)
		if err != nil {
			return fail(fmt.Errorf("failed to unmarshal chess meta: %w", err))
		}
		views = append(views, cv)
	}

	slog.InfoContext(ctx, "retrieved chess meta views", "views", views, "zSetName", zSetName, "page", page)
	return views, nil
}

func GetChessStateCount(ctx context.Context, rdb *Redis) (int64, error) {
	conn := rdb.Cache.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, rdb.GamesZSet); err != nil {
		return 0, err
	}

	count, err := redis.Int64(conn.Do("ZCARD", rdb.GamesZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to 'ZCARD' chess count: %w", err)
	}

	slog.InfoContext(ctx, "selected chess count", "count", count)
	return count, nil
}
