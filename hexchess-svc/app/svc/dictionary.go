package svc

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log/slog"
	"math"
	"math/rand"
	"strconv"
	"time"
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

func getUserGameZSet(id int64) string {
	return GamesZSet + "_user_" + strconv.FormatInt(id, 10)
}

var ErrNoChessState = errors.New("no chess state")

func GetChessState(ctx context.Context, rdb *redis.Pool, id string) (ChessState, error) {
	trace := ctx.Value(TraceKey)

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
	trace := ctx.Value(TraceKey)

	state.Touch = time.Now()
	touch := float64(state.Touch.Unix())
	fullID := "game:" + id

	data, err := state.Serialize()
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to serialize chess state: %w", err)
	}

	conn := rdb.Get()
	defer conn.Close()

	conn.Send("MULTI")
	conn.Send("SET", fullID, data)
	conn.Send("ZADD", GamesZSet, touch, fullID)
	if state.WhitePlayer != nil {
		conn.Send("ZADD", getUserGameZSet(state.WhitePlayer.ID), touch, fullID)
	}
	if state.BlackPlayer != nil {
		conn.Send("ZADD", getUserGameZSet(state.BlackPlayer.ID), touch, fullID)
	}
	if _, err = conn.Do("EXEC"); err != nil {
		return ChessState{}, fmt.Errorf("failed to set chess state: %w", err)
	}

	slog.Info("set chess state", "key", fullID, "trace", trace)
	return state, nil
}

func ExpireChessStates(ctx context.Context, conn redis.Conn, zSetName string) error {
	return ExpireChessStatesBefore(ctx, conn, zSetName, time.Now().Add(-GameExpireFinished))
}

func ExpireChessStatesBefore(ctx context.Context, conn redis.Conn, zSetName string, expireBefore time.Time) error {
	trace := ctx.Value(TraceKey)

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

func GetAllChessViews(ctx context.Context, rdb *redis.Pool, page, count int) ([]ChessView, error) {
	return GetChessViews(ctx, rdb, GamesZSet, page, count)
}

func GetChessViews(ctx context.Context, rdb *redis.Pool, zSetName string, page, count int) ([]ChessView, error) {
	trace := ctx.Value(TraceKey)

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

var ErrNoSession = errors.New("session not found")

func GetSession(ctx context.Context, rdb *redis.Pool, sessionID string) (PlayerState, error) {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	fullID := "session:" + sessionID
	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return PlayerState{}, ErrNoSession
	}
	if err != nil {
		slog.Error("failed to get session", "sessionID", sessionID, "trace", trace)
		return PlayerState{}, err
	}

	player, err := PlayerDeserialize(data)
	if err != nil {
		return PlayerState{}, err
	}

	slog.Info("selected session", "sessionID", sessionID, "player", player, "trace", trace)
	return player, nil
}

func SetSession(ctx context.Context, rdb *redis.Pool, sessionID string, player PlayerState, expiry time.Duration) error {
	fullID := "session:" + sessionID
	data, err := player.Serialize()
	if err != nil {
		return err
	}
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("SETEX", fullID, int(expiry.Seconds()), data); err != nil {
		slog.Error("failed to set session", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("set session", "sessionID", sessionID, "player", player, "trace", trace)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb *redis.Pool, sessionID string, expiry time.Duration) error {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("EXPIRE", "session:"+sessionID, int(expiry.Seconds())); err != nil {
		slog.Error("failed to update session expiry", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("updated session expiry", "sessionID", sessionID, "trace", trace)
	return nil
}

func DeleteSession(ctx context.Context, rdb *redis.Pool, sessionID string) error {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", "session:"+sessionID); err != nil {
		slog.Error("failed to delete session", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("deleted session", "sessionID", sessionID, "trace", trace)
	return nil
}

func AddUser(ctx context.Context, rdb *redis.Pool, id string) error {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", ActiveUsersZSet, "NX", "CH", float64(time.Now().UnixMilli()), id); err != nil {
		slog.Error("failed to add user", "id", id, "trace", trace, "err", err)
		return err
	}

	slog.Info("added user", "id", id, "trace", trace)
	return nil
}

func RemoveUser(ctx context.Context, rdb *redis.Pool, id string) error {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREM", ActiveUsersZSet, id); err != nil {
		slog.Info("failed to remove user", "err", err, "trace", trace)
	}

	slog.Info("removed user", "id", id, "trace", trace)
	return nil
}

func ExpireUsers(ctx context.Context, conn redis.Conn) error {
	return ExpireUsersBefore(ctx, conn, time.Now().Add(-UserExpireFinished).UnixMilli())
}

func ExpireUsersBefore(ctx context.Context, conn redis.Conn, expireBefore int64) error {
	keys, err := redis.Strings(conn.Do("ZREMRANGEBYSCORE", ActiveUsersZSet, "-inf", expireBefore))
	if err != nil {
		slog.Error("failed to expire users", "expireBefore", expireBefore, "trace", ctx.Value(TraceKey))
		return err
	}
	if len(keys) > 0 {
		slog.Info("expired users with keys", "keys", keys)
	}
	return nil
}

func GetUsersCount(ctx context.Context, rdb *redis.Pool) (int64, error) {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if err := ExpireUsers(ctx, conn); err != nil {
		return 0, err
	}
	count, err := redis.Int64(conn.Do("ZCARD", ActiveUsersZSet))
	if err != nil {
		return 0, fmt.Errorf("failed to select users count: %w", err)
	}

	slog.Info("selected users count", "count", count, "trace", trace)
	return count, err
}

func GetChessStateCount(ctx context.Context, rdb *redis.Pool) (int64, error) {
	trace := ctx.Value(TraceKey)
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

type IncrLbChangeSet struct {
	ID      int64
	EloDiff float64
}

func IncrLeaderboard(ctx context.Context, rdb *redis.Pool, csList ...IncrLbChangeSet) error {
	trace := ctx.Value(TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	conn.Send("MULTI")
	for _, cs := range csList {
		conn.Send("ZINCRBY", LeaderboardZSet, cs.EloDiff, cs.ID)
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return fmt.Errorf("failed to increment leaderboard user: %w", err)
	}

	slog.Info("incremented leaderboard user", "csList", csList, "trace", trace)
	return nil
}

type Leaderboard struct {
	Users     []RankedUser `json:"users"`
	PageCount int          `json:"pageCount"`
}

func GetLeaderboardRank(ctx context.Context, rdb *redis.Pool, id int64) (int, error) {
	trace := ctx.Value(TraceKey)

	fail := func(str string, err error) (int, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get leaderboard rank", "id", id, "trace", trace, "err", err)
		return 0, err
	}

	conn := rdb.Get()
	defer conn.Close()

	rank, err := redis.Int(conn.Do("ZREVRANK", LeaderboardZSet, id))
	if errors.Is(err, redis.ErrNil) {
		if _, err := conn.Do("ZINCRBY", LeaderboardZSet, StartElo, id); err != nil {
			return fail("failed to incr rank", err)
		}
		if rank, err = redis.Int(conn.Do("ZREVRANK", LeaderboardZSet, id)); err != nil {
			return fail("failed to get rank", err)
		}
	}
	if err != nil {
		return fail("failed to get rank", err)
	}

	slog.Info("retrieved leaderboard rank", "trace", trace, "id", id, "rank", rank+1)
	return rank + 1, nil
}

func GetLeaderboard(ctx context.Context, rdb *redis.Pool, startRank, count int) (Leaderboard, error) {
	trace := ctx.Value(TraceKey)

	fail := func(str string, err error) (Leaderboard, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to fetch leaderboard", "trace", trace, "startRank", startRank, "count", count, "err", err)
		return Leaderboard{}, err
	}

	conn := rdb.Get()
	defer conn.Close()

	end := startRank - 1 + count
	ids, err := redis.Strings(conn.Do("ZREVRANGE", LeaderboardZSet, int64(startRank), int64(end)))
	if err != nil {
		return fail("failed to get leaderboard", err)
	}
	elemCount, err := redis.Int64(conn.Do("ZCOUNT", LeaderboardZSet, "-inf", "+inf"))
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

func GetLeaderboardPage(ctx context.Context, rdb *redis.Pool, page, perPage int) (Leaderboard, error) {
	trace := ctx.Value(TraceKey)

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, offset, perPage)
	dynLog("retrieved leaderboard page", err, "trace", trace, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}
