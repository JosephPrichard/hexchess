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

func (svc *HexchessServices) GetSession(ctx context.Context, sessionID string) (PlayerState, error) {
	sessionKey := svc.makeSessionKey(sessionID)
	data, err := svc.redis.Cache.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return PlayerState{}, ErrSessionNotFound
		}
		return PlayerState{}, fmt.Errorf("get session %s: %w", sessionID, err)
	}

	player, err := UnmarshalPlayer(data)
	if err != nil {
		return PlayerState{}, fmt.Errorf("unmarshal session: %w", err)
	}
	slog.InfoContext(ctx, "selected session", "sessionID", sessionID, "player", player)
	return player, nil
}

type SessionInst struct {
	SessionID string
	Player    PlayerState
	Expiry    time.Duration
}

func (svc *HexchessServices) SetSessions(ctx context.Context, insts ...SessionInst) error {
	slog.InfoContext(ctx, "setting sessions", "insts", insts)

	pipe := svc.redis.Cache.TxPipeline()

	for _, inst := range insts {
		data, err := MarshalPlayer(inst.Player)
		if err != nil {
			return err
		}
		sessionKey := svc.makeSessionKey(inst.SessionID)
		pipe.SetEx(ctx, sessionKey, data, inst.Expiry)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set many sessions: %w", err)
	}
	return nil
}

func (svc *HexchessServices) UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error {
	sessionKey := svc.makeSessionKey(sessionID)
	if err := svc.redis.Cache.Expire(ctx, sessionKey, expiry).Err(); err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func (svc *HexchessServices) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := svc.makeSessionKey(sessionID)
	if err := svc.redis.Cache.Del(ctx, sessionKey).Err(); err != nil {
		return fmt.Errorf("delete session=%s: %w", sessionID, err)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
