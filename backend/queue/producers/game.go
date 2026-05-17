package producers

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"hexchess-svc/model"
	"log/slog"
)

type RedisXAdder interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
}

func PublishFinishGameEvent(ctx context.Context, xadder RedisXAdder, finishGameStreamKey string, finishedGame model.FinishedGame) error {
	bytes, err := model.MarshalFinishedGame(finishedGame)
	if err != nil {
		return fmt.Errorf("marshal finish game event: %w", err)
	}

	xArgs := &redis.XAddArgs{
		Stream: finishGameStreamKey,
		Values: map[string]any{"payload": string(bytes)},
	}
	msgID, err := xadder.XAdd(ctx, xArgs).Result()
	if err != nil {
		return fmt.Errorf("xadd finished game event %+v: %w", finishedGame, err)
	}

	slog.InfoContext(ctx, "pushed finished game event", "existingID", msgID, "gameID", finishedGame.GameID)
	return nil
}
