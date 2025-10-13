package dal

import (
	"context"
	"errors"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/app/util"
	"log/slog"
	"time"
)

const (
	GamesZSet       = "games"
	LeaderboardZSet = "leaderboard"
	ActiveUsersZSet = "active_users"
)

var ErrNoSession = errors.New("session not found")

func GetSession(ctx context.Context, rdb *redis.Pool, sessionID string) (PlayerState, error) {
	trace := ctx.Value(util.TraceKey)
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
	data, err := player.Serialize()
	if err != nil {
		return err
	}

	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	fullID := "session:" + sessionID

	if _, err := conn.Do("SETEX", fullID, int(expiry.Seconds()), data); err != nil {
		slog.Error("failed to set session", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("set session", "sessionID", sessionID, "player", player, "trace", trace)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb *redis.Pool, sessionID string, expiry time.Duration) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	fullID := "session:" + sessionID

	if _, err := conn.Do("EXPIRE", fullID, int(expiry.Seconds())); err != nil {
		slog.Error("failed to update session expiry", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("updated session expiry", "sessionID", sessionID, "trace", trace)
	return nil
}

func DeleteSession(ctx context.Context, rdb *redis.Pool, sessionID string) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", "session:"+sessionID); err != nil {
		slog.Error("failed to delete session", "sessionID", sessionID, "trace", trace, "err", err)
		return err
	}

	slog.Info("deleted session", "sessionID", sessionID, "trace", trace)
	return nil
}
