package game

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/proto/pb"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/event"
	"log/slog"
)

func PublishGameCountEvent(ctx context.Context, publisher pubsub.Publisher, count int64) {
	bytes, err := json.Marshal(event.CountEvent{Kind: event.GamesCountEvent, Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal count event", "error", err)
		return
	}
	publisher.Publish(ctx, event.GamesCountChannel, "GLOBAL", bytes)
}

func PublishGamesEvent(ctx context.Context, publisher pubsub.Publisher, output *pb.GameOutput) {
	slog.InfoContext(ctx, "PubSub: publishing game event", "gameId", output.GameId)

	bytes, err := output.MarshalVT()
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal game message", "error", err)
		return
	}
	publisher.Publish(ctx, event.GamesChannel, output.GameId, bytes)
}
