package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/infra"
	"log/slog"
	"time"

	"github.com/gomodule/redigo/redis"
)

var ErrSessionNotFound = errors.New("session not found")

func GetSession(ctx context.Context, rdb *infra.Redis, sessionID string) (PlayerState, error) {
	var p PlayerState

	conn := rdb.Cache.Get()
	defer conn.Close()

	fullID := "session:" + sessionID
	data, err := redis.Bytes(conn.Do("GET", fullID))
	if errors.Is(err, redis.ErrNil) {
		return p, ErrSessionNotFound
	}
	if err != nil {
		return p, fmt.Errorf("get session %s: %w", sessionID, err)
	}

	if p, err = UnmarshalPlayer(data); err != nil {
		return p, fmt.Errorf("unmarshal session: %w", err)
	}
	slog.InfoContext(ctx, "selected session", "sessionID", sessionID, "player", p)
	return p, nil
}

func SetSession(ctx context.Context, rdb *infra.Redis, sessionID string, player PlayerState, expiry time.Duration) error {
	data, err := MarshalPlayer(&player)
	if err != nil {
		return err
	}

	conn := rdb.Cache.Get()
	defer conn.Close()
	fullID := "session:" + sessionID

	if _, err := conn.Do("SETEX", fullID, int(expiry.Seconds()), data); err != nil {
		return err
	}
	slog.InfoContext(ctx, "set session", "sessionID", sessionID, "player", player)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb *infra.Redis, sessionID string, expiry time.Duration) error {
	conn := rdb.Cache.Get()
	defer conn.Close()

	fullID := "session:" + sessionID

	if _, err := conn.Do("EXPIRE", fullID, int(expiry.Seconds())); err != nil {
		return err
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func DeleteSession(ctx context.Context, rdb *infra.Redis, sessionID string) error {
	conn := rdb.Cache.Get()
	defer conn.Close()

	if _, err := conn.Do("DEL", "session:"+sessionID); err != nil {
		return err
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
