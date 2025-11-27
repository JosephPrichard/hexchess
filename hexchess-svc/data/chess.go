package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strconv"
	"time"
)

func getUserGameZSet(rdb *Redis, id int64) string {
	return rdb.GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb *Redis, id string) (ChessState, error) {
	fail := func(str string, err error) (ChessState, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to get chess state", "err", err, "id", id)
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

	state, err := UnmarshalChessState(data)
	if err != nil {
		return fail("failed to deserialize chess state", err)
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
		return state, fmt.Errorf("failed to set chess state: %w", err)
	}

	slog.InfoContext(ctx, "set chess state", "key", fullID, "touch", touch)
	return state, nil
}

const GameExpireFinished = 1 * time.Hour

func ExpireChessStates(ctx context.Context, conn redis.Conn, zSetName string) error {
	fail := func(str string, err error) error {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to expire chess states", "err", err, "zSetName", zSetName)
		return err
	}
	expireBefore := time.Now().Add(-GameExpireFinished)

	keys, err := redis.Values(conn.Do("ZRANGEBYSCORE", zSetName, "-inf", expireBefore.Unix()))
	if err != nil {
		return fail("failed to retrieve expired states", err)
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
		return fail("failed to remove expired states", err)
	}

	keyStrs := make([]string, 0, len(keys))
	for _, key := range keys {
		keyStrs = append(keyStrs, fmt.Sprintf("%s", key))
	}
	slog.InfoContext(ctx, "expired chess states", "zSetName", zSetName, "keys", keyStrs, "expireBefore", expireBefore)
	return err
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

	fail := func(str string, err error) ([]ChessMeta, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to get chess views", "err", err, "zSetName", zSetName, "page", page, "count", count)
		return nil, err
	}
	conn := rdb.Primary.Get()
	defer conn.Close()

	if err := ExpireChessStates(ctx, conn, zSetName); err != nil {
		return fail("failed to expire chess states", err)
	}
	elements, err := redis.Values(conn.Do("ZREVRANGE", zSetName, left, right))
	if err != nil {
		return fail("failed to get chess id", err)
	}
	var bytesList [][]byte
	if len(elements) > 0 {
		bytesList, err = redis.ByteSlices(conn.Do("MGET", elements...))
		if err != nil {
			return fail("failed to get chess states", err)
		}
	}

	views := make([]ChessMeta, 0)
	for _, bytes := range bytesList {
		cv, err := UnmarshalChessMeta(bytes)
		if err != nil {
			return fail("failed to unmarshal chess debug with key", err)
		}
		views = append(views, cv)
	}
	slog.InfoContext(ctx, "retrieved chess meta views", "views", views, "zSetName", zSetName, "page", page)
	return views, nil
}

func GetChessStateCount(ctx context.Context, rdb *Redis) (int64, error) {
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
