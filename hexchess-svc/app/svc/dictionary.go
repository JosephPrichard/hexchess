package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	GamesZSet       = "games"
	LeaderboardZSet = "leaderboard"
	ActiveUsersZSet = "active_users"
)

var (
	UserExpireFinished = 2 * time.Minute
	GameExpireFinished = 1 * time.Hour
	TempSessionExpire  = 1 * time.Minute
	Rand               = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func GetUserGameZSet(id int64) string {
	return GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

func GetRoom(ctx context.Context, client *redis.Client, id string) (ChessState, error) {
	trace := ctx.Value(TraceKey)

	if err := ExpireRooms(ctx, client, GamesZSet); err != nil {
		return ChessState{}, err
	}
	fullID := "game:" + id

	data, err := client.Get(ctx, fullID).Bytes()
	if errors.Is(err, redis.Nil) {
		return ChessState{}, nil
	} else if err != nil {
		slog.Error("failed to get room", "id", id, "err", err, "trace", trace)
		return ChessState{}, err
	}

	room, err := ChessDeserialize(data)
	if err != nil {
		return ChessState{}, err
	}
	slog.Info("selected session", "ud", id, "room", room, "trace", trace)
	return room, nil
}

func SetRoom(ctx context.Context, client *redis.Client, id string, room ChessState) (ChessState, error) {
	trace := ctx.Value(TraceKey)

	room.Touch = time.Now()
	fullID := "game:" + id
	data, err := room.Serialize()
	if err != nil {
		return ChessState{}, err
	}

	pipe := client.TxPipeline()
	pipe.Set(ctx, fullID, data, 0)
	pipe.ZAdd(ctx, GamesZSet, redis.Z{Score: float64(room.Touch.Unix()), Member: fullID})

	if room.WhitePlayer != nil {
		pipe.ZAdd(ctx, GetUserGameZSet(room.WhitePlayer.ID), redis.Z{Score: float64(room.Touch.Unix()), Member: fullID})
	}
	if room.BlackPlayer != nil {
		pipe.ZAdd(ctx, GetUserGameZSet(room.BlackPlayer.ID), redis.Z{Score: float64(room.Touch.Unix()), Member: fullID})
	}

	_, err = pipe.Exec(ctx)
	slog.Log(nil, dynLevel(err), "set room", "id", id, "room", room, "err", err, "trace", trace)
	return room, err
}

func ExpireRooms(ctx context.Context, client *redis.Client, zSetName string) error {
	return ExpireRoomsBefore(ctx, client, zSetName, time.Now().Add(-GameExpireFinished).UnixMilli())
}

func ExpireRoomsBefore(ctx context.Context, client *redis.Client, zSetName string, expireBefore int64) error {
	trace := ctx.Value(TraceKey)

	by := &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(expireBefore, 10)}
	keys, err := client.ZRangeByScore(ctx, zSetName, by).Result()
	if err != nil {
		slog.Error("failed to retrieved expired rooms", "zSetName", zSetName, "expireBefore", expireBefore, "err", err, "trace", trace)
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	slog.Info("expiring rooms with keys", "keys", keys)

	pipe := client.TxPipeline()
	pipe.Del(ctx, keys...)
	pipe.ZRem(ctx, zSetName, keys)

	_, err = pipe.Exec(ctx)
	slog.Log(nil, dynLevel(err), "failed to expire rooms", "zSetName", zSetName, "expireBefore", expireBefore, "err", err, "trace", trace)
	return err
}

func GetUserChessViews(ctx context.Context, client *redis.Client, userID int64) ([]ChessView, error) {
	return GetChessViews(ctx, client, GetUserGameZSet(userID), 1, -1)
}

func GetAllChessViews(ctx context.Context, client *redis.Client, page, count int) ([]ChessView, error) {
	return GetChessViews(ctx, client, GamesZSet, page, count)
}

func GetChessViews(ctx context.Context, client *redis.Client, zSetName string, page, count int) ([]ChessView, error) {
	trace := ctx.Value(TraceKey)

	fail := func(err error) ([]ChessView, error) {
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

	if err := ExpireRooms(ctx, client, zSetName); err != nil {
		return fail(err)
	}

	elements, err := client.ZRevRange(ctx, zSetName, left, right).Result()
	if err != nil {
		return fail(err)
	}

	var dataList []interface{}

	if len(elements) > 0 {
		if dataList, err = client.MGet(ctx, elements...).Result(); err != nil {
			return fail(err)
		}
	}

	var views []ChessView
	for i, data := range dataList {
		if data == nil {
			continue
		}
		str, ok := data.(string)
		if !ok {
			return fail(fmt.Errorf("unexpected type: %T for key: %v, should be string", data, elements[i]))
		}
		cv, err := ChessViewDeserialize(str)
		if err != nil {
			return fail(fmt.Errorf("failed to deserialize chess view with key: %s, err: %w", elements[i], err))
		}
		views = append(views, cv)
	}

	slog.Info("retrieved chess views", "count", len(views), "set", zSetName, "page", page, "trace", trace)
	return views, nil
}

var ErrNoSession = errors.New("session not found")

func GetSession(ctx context.Context, client *redis.Client, sessionID string) (PlayerState, error) {
	trace := ctx.Value(TraceKey)

	fullID := "session:" + sessionID
	data, err := client.Get(ctx, fullID).Bytes()
	if errors.Is(err, redis.Nil) {
		return PlayerState{}, ErrNoSession
	}
	if err != nil {
		slog.Error("failed to get session", "sessionID", sessionID, "err", err, "trace", trace)
		return PlayerState{}, err
	}
	player, err := PlayerDeserialize(data)
	if err != nil {
		return PlayerState{}, err
	}
	slog.Info("selected session", "sessionID", sessionID, "player", player, "trace", trace)
	return player, nil
}

func SetSession(ctx context.Context, client *redis.Client, sessionID string, player PlayerState, expiry time.Duration) error {
	trace := ctx.Value(TraceKey)

	fullID := "session:" + sessionID
	data, err := player.Serialize()
	if err != nil {
		return err
	}
	if err := client.SetEx(ctx, fullID, data, expiry).Err(); err != nil {
		slog.Error("failed to set session", "sessionID", sessionID, "err", err)
		return err
	}
	slog.Info("set session", "sessionID", sessionID, "player", player, "trace", trace)
	return nil
}

func UpdateSessionEx(ctx context.Context, client *redis.Client, sessionID string, expiry time.Duration) error {
	err := client.Expire(ctx, "session:"+sessionID, expiry).Err()
	slog.Log(nil, dynLevel(err), "updated session", "sessionID", sessionID, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func DeleteSession(ctx context.Context, client *redis.Client, sessionID string) error {
	err := client.Del(ctx, "session:"+sessionID).Err()
	slog.Log(nil, dynLevel(err), "deleted session", "sessionID", sessionID, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func IncrLeaderboardUser(ctx context.Context, client *redis.Client, id int64, elo float64) error {
	err := client.ZIncrBy(ctx, LeaderboardZSet, elo, strconv.FormatInt(id, 10)).Err()
	slog.Log(nil, dynLevel(err), "incremented user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func AddUser(ctx context.Context, client *redis.Client, id string) error {
	by := redis.Z{Score: float64(time.Now().UnixMilli()), Member: id}
	err := client.ZAdd(ctx, ActiveUsersZSet, by).Err()
	slog.Log(nil, dynLevel(err), "added user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func RemoveUser(ctx context.Context, client *redis.Client, id string) error {
	err := client.ZRem(ctx, ActiveUsersZSet, id).Err()
	slog.Log(nil, dynLevel(err), "removed user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func ExpireUsers(ctx context.Context, client *redis.Client) error {
	return ExpireUsersBefore(ctx, client, time.Now().Add(-UserExpireFinished).UnixMilli())
}

func ExpireUsersBefore(ctx context.Context, client *redis.Client, expireBefore int64) error {
	by := &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(expireBefore, 10)}
	keys, err := client.ZRangeByScore(ctx, ActiveUsersZSet, by).Result()
	if err != nil {
		slog.Error("failed to expire users", "expireBefore", expireBefore, "err", err, "trace", ctx.Value(TraceKey))
		return err
	}
	if len(keys) > 0 {
		slog.Info("expiring users with keys", "keys", keys)
		client.ZRem(ctx, ActiveUsersZSet, keys)
	}
	return nil
}

func GetUsersCount(ctx context.Context, client *redis.Client) (int64, error) {
	if err := ExpireUsers(ctx, client); err != nil {
		return 0, err
	}
	count, err := client.ZCard(ctx, ActiveUsersZSet).Result()
	slog.Log(nil, dynLevel(err), "selected users count", "count", count, "err", err, "trace", ctx.Value(TraceKey))
	return count, err
}

func GetRoomsCount(ctx context.Context, client *redis.Client) (int64, error) {
	if err := ExpireRooms(ctx, client, GamesZSet); err != nil {
		return 0, err
	}
	count, err := client.ZCard(ctx, GamesZSet).Result()
	slog.Log(nil, dynLevel(err), "selected users count", "count", count, "err", err, "trace", ctx.Value(TraceKey))
	return count, err
}
