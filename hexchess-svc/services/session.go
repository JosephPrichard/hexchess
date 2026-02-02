package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

func (svc *Services) GetSession(ctx context.Context, sessionID string) (PlayerState, error) {
	var p PlayerState

	fullID := "session:" + sessionID
	data, err := svc.Redis.Cache.Get(ctx, fullID).Bytes()
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

type SessInst struct {
	SessionID string
	Player    PlayerState
	Expiry    time.Duration
}

func (svc *Services) SetSessions(ctx context.Context, insts ...SessInst) error {
	slog.InfoContext(ctx, "setting sessions", "insts", insts)

	pipe := svc.Redis.Cache.TxPipeline()

	for _, inst := range insts {
		data, err := MarshalPlayer(inst.Player)
		if err != nil {
			return err
		}
		fullID := "session:" + inst.SessionID
		pipe.SetEx(ctx, fullID, data, inst.Expiry)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set many sessions: %w", err)
	}
	return nil
}

func (svc *Services) UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error {
	fullID := "session:" + sessionID
	if err := svc.Redis.Cache.Expire(ctx, fullID, expiry).Err(); err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func (svc *Services) DeleteSession(ctx context.Context, sessionID string) error {
	fullID := "session:" + sessionID
	if err := svc.Redis.Cache.Del(ctx, fullID).Err(); err != nil {
		return fmt.Errorf("delete session=%s: %w", sessionID, err)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
