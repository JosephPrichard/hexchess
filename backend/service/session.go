package svc

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

func (services *HexchessServices) GetSession(ctx context.Context, sessionID string) (model.PlayerState, error) {
	sessionKey := cache.FmtSessionKey(sessionID)

	slog.InfoContext(ctx, "getting session", "sessionID", sessionID, "sessionKey", sessionKey)

	bytes, err := services.redis.PrimaryClient.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return model.PlayerState{}, ErrSessionNotFound
		}
		return model.PlayerState{}, serrors.New("get session", err, "sessionID", sessionID)
	}

	player, err := model.UnmarshalPlayer(bytes)
	if err != nil {
		return model.PlayerState{}, serrors.New("unmarshal session", err)
	}
	slog.InfoContext(ctx, "retrieved session", "sessionID", sessionID, "sessionKey", sessionKey, "player", player)
	return player, nil
}

type SessionInst struct {
	SessionID string
	Player    model.PlayerState
	Expiry    time.Duration
}

func (services *HexchessServices) SetSessions(ctx context.Context, insts ...SessionInst) error {
	slog.InfoContext(ctx, "setting sessions", "insts", insts)

	pipe := services.redis.PrimaryClient.Pipeline()

	for _, inst := range insts {
		data, err := model.MarshalPlayer(inst.Player)
		if err != nil {
			return err
		}
		sessionKey := cache.FmtSessionKey(inst.SessionID)
		pipe.SetEx(ctx, sessionKey, data, inst.Expiry)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.New("set many sessions", err)
	}
	return nil
}

func (services *HexchessServices) UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error {
	sessionKey := cache.FmtSessionKey(sessionID)
	if err := services.redis.PrimaryClient.Expire(ctx, sessionKey, expiry).Err(); err != nil {
		return serrors.New("update session expiry", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func (services *HexchessServices) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := cache.FmtSessionKey(sessionID)
	if err := services.redis.PrimaryClient.Del(ctx, sessionKey).Err(); err != nil {
		return serrors.New("delete session", err, "sessionID", sessionID)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
