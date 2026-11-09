package consumers

import (
	"context"
	"log/slog"

	"uuid"

	"github.com/redis/go-redis/v9"
)

func extractUUID(ctx context.Context, msg redis.XMessage, key string) uuid.UUID {
	targetID := uuid.NewV7()

	targetIDStr, _ := msg.Values[key].(string)
	parsedUUID, err := uuid.Parse(targetIDStr)
	if err != nil {
		slog.WarnContext(ctx, "received invalid UUID on stream", "key", key, "targetIDStr", targetIDStr, "error", err)
	} else {
		targetID = parsedUUID
	}

	return targetID
}
