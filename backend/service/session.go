package svc

import (
	"context"
	"errors"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

func (services *HexchessServices) GetSession(ctx context.Context, sessionID string) (model.PlayerState, error) {
	sessionKey := makeSessionKey(sessionID)
	bytes, err := services.redis.Cache.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return model.PlayerState{}, ErrSessionNotFound
		}
		return model.PlayerState{}, serrors.Wrap("get session", err, "sessionID", sessionID)
	}

	player, err := model.UnmarshalPlayer(bytes)
	if err != nil {
		return model.PlayerState{}, serrors.Wrap("unmarshal session", err)
	}
	slog.InfoContext(ctx, "selected session", "sessionID", sessionID, "player", player)
	return player, nil
}

type SessionInst struct {
	SessionID string
	Player    model.PlayerState
	Expiry    time.Duration
}

func (services *HexchessServices) SetSessions(ctx context.Context, insts ...SessionInst) error {
	slog.InfoContext(ctx, "setting sessions", "insts", insts)

	pipe := services.redis.Cache.Pipeline()

	for _, inst := range insts {
		data, err := model.MarshalPlayer(inst.Player)
		if err != nil {
			return err
		}
		sessionKey := makeSessionKey(inst.SessionID)
		pipe.SetEx(ctx, sessionKey, data, inst.Expiry)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.Wrap("set many sessions", err)
	}
	return nil
}

func (services *HexchessServices) UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error {
	sessionKey := makeSessionKey(sessionID)
	if err := services.redis.Cache.Expire(ctx, sessionKey, expiry).Err(); err != nil {
		return serrors.Wrap("update session expiry", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func (services *HexchessServices) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := makeSessionKey(sessionID)
	if err := services.redis.Cache.Del(ctx, sessionKey).Err(); err != nil {
		return serrors.Wrap("delete session", err, "sessionID", sessionID)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
