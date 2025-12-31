package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

func GetSession(ctx context.Context, rdb *db.Rdb, sessionID string) (PlayerState, error) {
	var p PlayerState

	fullID := "session:" + sessionID
	data, err := rdb.Cache.Get(ctx, fullID).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return p, ErrSessionNotFound
		}
		return p, fmt.Errorf("get session %s: %w", sessionID, err)
	}

	p, err = UnmarshalPlayer(data)
	if err != nil {
		return p, fmt.Errorf("unmarshal session: %w", err)
	}
	slog.InfoContext(ctx, "selected session", "sessionID", sessionID, "player", p)
	return p, nil
}

func SetSession(ctx context.Context, rdb *db.Rdb, sessionID string, player PlayerState, expiry time.Duration) error {
	data, err := MarshalPlayer(player)
	if err != nil {
		return err
	}
	fullID := "session:" + sessionID
	if err := rdb.Cache.SetEx(ctx, fullID, data, expiry).Err(); err != nil {
		return fmt.Errorf("set session: %w", err)
	}
	slog.InfoContext(ctx, "set session", "sessionID", sessionID, "player", player)
	return nil
}

func UpdateSessionEx(ctx context.Context, rdb *db.Rdb, sessionID string, expiry time.Duration) error {
	fullID := "session:" + sessionID
	if err := rdb.Cache.Expire(ctx, fullID, expiry).Err(); err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func DeleteSession(ctx context.Context, rdb *db.Rdb, sessionID string) error {
	fullID := "session:" + sessionID
	if err := rdb.Cache.Del(ctx, fullID).Err(); err != nil {
		return fmt.Errorf("delete session '%s': %w", sessionID, err)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
