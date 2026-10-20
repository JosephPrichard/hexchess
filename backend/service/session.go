package service

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/utils/errutil"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionService struct {
	redis cache.Redis
}

func NewSessionService(redis cache.Redis) *SessionService {
	return &SessionService{redis: redis}
}

func (service *SessionService) GetSession(ctx context.Context, sessionID string) (model.PlayerState, error) {
	sessionKey := cache.FmtSessionKey(sessionID)

	slog.InfoContext(ctx, "getting session", "sessionID", sessionID, "sessionKey", sessionKey)

	bytes, err := service.redis.PrimaryClient.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return model.PlayerState{}, ErrSessionNotFound
		}
		return model.PlayerState{}, errutil.New("get session", err, "sessionID", sessionID)
	}

	player, err := model.UnmarshalPlayer(bytes)
	if err != nil {
		return model.PlayerState{}, errutil.New("unmarshal session", err)
	}
	slog.InfoContext(ctx, "retrieved session", "sessionID", sessionID, "sessionKey", sessionKey, "player", player)
	return player, nil
}

type SessionInst struct {
	SessionID string
	Player    model.PlayerState
	Expiry    time.Duration
}

func (service *SessionService) SetSessions(ctx context.Context, insts ...SessionInst) error {
	slog.InfoContext(ctx, "setting sessions", "insts", insts)

	pipe := service.redis.PrimaryClient.Pipeline()

	for _, inst := range insts {
		data, err := model.MarshalPlayer(inst.Player)
		if err != nil {
			return err
		}
		sessionKey := cache.FmtSessionKey(inst.SessionID)
		pipe.SetEx(ctx, sessionKey, data, inst.Expiry)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return errutil.New("set many sessions", err)
	}
	return nil
}

func (service *SessionService) UpdateSessionEx(ctx context.Context, sessionID string, expiry time.Duration) error {
	sessionKey := cache.FmtSessionKey(sessionID)
	if err := service.redis.PrimaryClient.Expire(ctx, sessionKey, expiry).Err(); err != nil {
		return errutil.New("update session expiry", err)
	}
	slog.InfoContext(ctx, "updated session expiry", "sessionID", sessionID)
	return nil
}

func (service *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := cache.FmtSessionKey(sessionID)
	if err := service.redis.PrimaryClient.Del(ctx, sessionKey).Err(); err != nil {
		return errutil.New("delete session", err, "sessionID", sessionID)
	}
	slog.InfoContext(ctx, "deleted session", "sessionID", sessionID)
	return nil
}
