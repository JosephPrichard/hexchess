package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log/slog"
	"time"
)

const LeaderboardZSet = "leaderboard"

var ErrSessionNotFound = errors.New("session not found")

func GetSession(ctx context.Context, rdb Redis, sessionID string) (PlayerState, error) {
	conn := rdb.Primary.Get()
	defer conn.Close()

	fullID := "session:" + sessionID
	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return PlayerState{}, ErrSessionNotFound
	}
	if err != nil {
		return PlayerState{}, fmt.Errorf("failed to get session %s: %w", sessionID, err)
	}

	player, err := UnmarshalPlayer(data)
	if err != nil {
		return PlayerState{}, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	slog.InfoContext(ctx, "selected session", "sessionID", sessionID, "player", player)
	return player, nil
}

func SetSession(ctx context.Context, rdb Redis, sessionID string, player PlayerState, expiry time.Duration) error {
	data, err := MarshalPlayer(&player)
	if err != nil {
		return err
	}

	conn := rdb.Primary.Get()
	defer conn.Close()
	fullID := "session:" + sessionID

	if _, err := conn.Do("SETEX", fullID, int(expiry.Seconds()), data); err != nil {
		return err
	}
	slog.InfoContext(ctx, "set session", "sessionID", sessionID, "player", player)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb Redis, sessionID string, expiry time.Duration) error {
	conn := rdb.Primary.Get()
	defer conn.Close()

	fullID := "session:" + sessionID

	if _, err := conn.Do("EXPIRE", fullID, int(expiry.Seconds())); err != nil {
		return err
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func DeleteSession(ctx context.Context, rdb Redis, sessionID string) error {
	conn := rdb.Primary.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", "session:"+sessionID); err != nil {
		return err
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
