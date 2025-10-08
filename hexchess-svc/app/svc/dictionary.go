package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
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

func GetChessState(ctx context.Context, rdb *redis.Client, id string) (ChessState, error) {
	trace := ctx.Value(TraceKey)

	fail := func(str string, err error) (ChessState, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get chess state", "id", id, "err", err, "trace", trace)
		return ChessState{}, err
	}

	if err := ExpireChessStates(ctx, rdb, GamesZSet); err != nil {
		return fail("failed to expire chess states", err)
	}
	fullID := "game:" + id

	data, err := rdb.Get(ctx, fullID).Bytes()
	if errors.Is(err, redis.Nil) {
		return ChessState{}, nil
	} else if err != nil {
		return fail("failed to get chess state", err)
	}

	state, err := ChessDeserialize(data)
	if err != nil {
		return fail("failed to deserialize chess state", err)
	}
	slog.Info("selected session", "key", fullID, "id", state.ID, "trace", trace)
	return state, nil
}

func SetChessState(ctx context.Context, rdb *redis.Client, id string, state ChessState) (ChessState, error) {
	trace := ctx.Value(TraceKey)

	state.Touch = time.Now()
	touch := float64(state.Touch.Unix())
	fullID := "game:" + id

	data, err := state.Serialize()
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to serialize chess state: %w", err)
	}

	pipe := rdb.TxPipeline()
	pipe.Set(ctx, fullID, data, 0)
	pipe.ZAdd(ctx, GamesZSet, redis.Z{Score: touch, Member: fullID})

	if state.WhitePlayer != nil {
		pipe.ZAdd(ctx, GetUserGameZSet(state.WhitePlayer.ID), redis.Z{Score: touch, Member: fullID})
	}
	if state.BlackPlayer != nil {
		pipe.ZAdd(ctx, GetUserGameZSet(state.BlackPlayer.ID), redis.Z{Score: touch, Member: fullID})
	}

	_, err = pipe.Exec(ctx)
	slog.Log(nil, dynLevel(err), "set chess state", "key", fullID, "id", state.ID, "err", err, "trace", trace)
	return state, err
}

func ExpireChessStates(ctx context.Context, rdb *redis.Client, zSetName string) error {
	return ExpireChessStatesBefore(ctx, rdb, zSetName, time.Now().Add(-GameExpireFinished))
}

func ExpireChessStatesBefore(ctx context.Context, rdb *redis.Client, zSetName string, expireBefore time.Time) error {
	trace := ctx.Value(TraceKey)

	by := &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(expireBefore.Unix(), 10)}
	keys, err := rdb.ZRangeByScore(ctx, zSetName, by).Result()
	if err != nil {
		slog.Error("failed to retrieved expired chess states", "zSetName", zSetName, "err", err, "trace", trace)
		return fmt.Errorf("failed to retrieve expired chess states: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	pipe := rdb.TxPipeline()
	pipe.Del(ctx, keys...)
	pipe.ZRem(ctx, zSetName, keys)

	_, err = pipe.Exec(ctx)
	slog.Log(nil, dynLevel(err), "expired chess states", "zSetName", zSetName, "keys", keys, "expireBefore", expireBefore, "err", err, "trace", trace)
	return err
}

func GetUserChessViews(ctx context.Context, rdb *redis.Client, userID int64) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, GetUserGameZSet(userID), 1, -1)
}

func GetAllChessViews(ctx context.Context, rdb *redis.Client, page, count int) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, GamesZSet, page, count)
}

func GetChessViews(ctx context.Context, rdb *redis.Client, zSetName string, page, count int) ([]ChessView, error) {
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

	var dataList []interface{}

	if err := ExpireChessStates(ctx, rdb, zSetName); err != nil {
		return fail(fmt.Errorf("failed to expire chess states: %w", err))
	}
	elements, err := rdb.ZRevRange(ctx, zSetName, left, right).Result()
	if err != nil {
		return fail(fmt.Errorf("failed to get chess ids: %w", err))
	}
	if len(elements) > 0 {
		if dataList, err = rdb.MGet(ctx, elements...).Result(); err != nil {
			return fail(fmt.Errorf("failed to get chess states: %w", err))
		}
	}

	var views []ChessView
	for i, data := range dataList {
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

	slog.Info("retrieved chess views", "count", len(views), "zset", zSetName, "page", page, "trace", trace)
	return views, nil
}

var ErrNoSession = errors.New("session not found")

func GetSession(ctx context.Context, rdb *redis.Client, sessionID string) (PlayerState, error) {
	trace := ctx.Value(TraceKey)

	fullID := "session:" + sessionID
	data, err := rdb.Get(ctx, fullID).Bytes()
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

func SetSession(ctx context.Context, rdb *redis.Client, sessionID string, player PlayerState, expiry time.Duration) error {
	trace := ctx.Value(TraceKey)

	fullID := "session:" + sessionID
	data, err := player.Serialize()
	if err != nil {
		return err
	}
	if err := rdb.SetEx(ctx, fullID, data, expiry).Err(); err != nil {
		slog.Error("failed to set session", "sessionID", sessionID, "err", err)
		return err
	}
	slog.Info("set session", "sessionID", sessionID, "player", player, "trace", trace)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb *redis.Client, sessionID string, expiry time.Duration) error {
	err := rdb.Expire(ctx, "session:"+sessionID, expiry).Err()
	slog.Log(nil, dynLevel(err), "updated session", "sessionID", sessionID, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func DeleteSession(ctx context.Context, rdb *redis.Client, sessionID string) error {
	err := rdb.Del(ctx, "session:"+sessionID).Err()
	slog.Log(nil, dynLevel(err), "deleted session", "sessionID", sessionID, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func AddUser(ctx context.Context, rdb *redis.Client, id string) error {
	by := redis.Z{Score: float64(time.Now().UnixMilli()), Member: id}
	err := rdb.ZAdd(ctx, ActiveUsersZSet, by).Err()
	slog.Log(nil, dynLevel(err), "added user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func RemoveUser(ctx context.Context, rdb *redis.Client, id string) error {
	err := rdb.ZRem(ctx, ActiveUsersZSet, id).Err()
	slog.Log(nil, dynLevel(err), "removed user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func ExpireUsers(ctx context.Context, rdb *redis.Client) error {
	return ExpireUsersBefore(ctx, rdb, time.Now().Add(-UserExpireFinished).UnixMilli())
}

func ExpireUsersBefore(ctx context.Context, rdb *redis.Client, expireBefore int64) error {
	by := &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(expireBefore, 10)}
	keys, err := rdb.ZRangeByScore(ctx, ActiveUsersZSet, by).Result()
	if err != nil {
		slog.Error("failed to expire users", "expireBefore", expireBefore, "err", err, "trace", ctx.Value(TraceKey))
		return err
	}
	if len(keys) > 0 {
		slog.Info("expiring users with keys", "keys", keys)
		rdb.ZRem(ctx, ActiveUsersZSet, keys)
	}
	return nil
}

func GetUsersCount(ctx context.Context, rdb *redis.Client) (int64, error) {
	if err := ExpireUsers(ctx, rdb); err != nil {
		return 0, err
	}
	count, err := rdb.ZCard(ctx, ActiveUsersZSet).Result()
	slog.Log(nil, dynLevel(err), "selected users count", "count", count, "err", err, "trace", ctx.Value(TraceKey))
	return count, err
}

func GetChessStateCount(ctx context.Context, rdb *redis.Client) (int64, error) {
	if err := ExpireChessStates(ctx, rdb, GamesZSet); err != nil {
		return 0, err
	}
	count, err := rdb.ZCard(ctx, GamesZSet).Result()
	slog.Log(nil, dynLevel(err), "selected users count", "count", count, "err", err, "trace", ctx.Value(TraceKey))
	return count, err
}

func IncrLeaderboardUser(ctx context.Context, rdb *redis.Client, id int64, elo float64) error {
	err := rdb.ZIncrBy(ctx, LeaderboardZSet, elo, strconv.FormatInt(id, 10)).Err()
	slog.Log(nil, dynLevel(err), "incremented user", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

type Leaderboard struct {
	Users     []RankedUser `json:"users"`
	PageCount int          `json:"pageCount"`
}

func GetLeaderboardRank(ctx context.Context, rdb *redis.Client, id int64) (int, error) {
	trace := ctx.Value(TraceKey)
	strID := strconv.FormatInt(id, 10)

	fail := func(str string, err error) (int, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get leaderboard rank", "trace", trace, "id", id, "err", err)
		return 0, err
	}

	rank, err := rdb.ZRevRank(ctx, LeaderboardZSet, strID).Result()
	if errors.Is(err, redis.Nil) {
		if err := IncrLeaderboardUser(ctx, rdb, id, StartElo); err != nil {
			return fail("failed to incr rank", err)
		}
		rank, err = rdb.ZRevRank(ctx, LeaderboardZSet, strID).Result()
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return fail("failed to get rank", err)
	}

	slog.Info("retrieved leaderboard rank", "trace", trace, "id", id, "rank", int(rank)+1)
	return int(rank) + 1, nil
}

func GetLeaderboard(ctx context.Context, rdb *redis.Client, startRank, count int) (Leaderboard, error) {
	trace := ctx.Value(TraceKey)

	fail := func(str string, err error) (Leaderboard, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to fetch leaderboard", "trace", trace, "startRank", startRank, "count", count, "err", err)
		return Leaderboard{}, err
	}

	end := startRank - 1 + count
	ids, err := rdb.ZRevRange(ctx, LeaderboardZSet, int64(startRank), int64(end)).Result()
	if err != nil {
		return fail("failed to get leaderboard", err)
	}
	elemCount, err := rdb.ZCount(ctx, LeaderboardZSet, "-inf", "+inf").Result()
	if err != nil {
		return fail("failed to count leaderboard", err)
	}

	users := make([]RankedUser, 0, len(ids))
	for i, strID := range ids {
		id, err := strconv.ParseInt(strID, 10, 64)
		if err != nil {
			return fail("failed to parse ranked ID", err)
		}
		users = append(users, RankedUser{ID: id, Rank: int64(startRank + i + 1)})
	}

	pageCount := int((elemCount / int64(count)) + int64(math.Min(float64(elemCount%int64(count)), 1)))

	leaderboard := Leaderboard{Users: users, PageCount: pageCount}
	slog.Info("retrieved leaderboard", "trace", trace, "startRank", startRank, "count", count, "leaderboard", leaderboard)
	return leaderboard, nil
}

func GetLeaderboardPage(ctx context.Context, rdb *redis.Client, page, perPage int) (Leaderboard, error) {
	trace := ctx.Value(TraceKey)

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, offset, perPage)
	slog.Log(nil, dynLevel(err), "retrieved leaderboard page", "trace", trace, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}
