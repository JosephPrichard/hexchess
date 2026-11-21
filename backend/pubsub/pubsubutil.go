package pubsub

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub/event"
	"log/slog"
	"strconv"
)

func publishCountEvent(ctx context.Context, publisher Publisher, channel string, count int64) {
	slog.InfoContext(ctx, "PubSub: publishing count event", "channel", channel, "count", count)

	bytes, err := json.Marshal(event.CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal count event", "error", err)
		return
	}
	publisher.Publish(ctx, channel, "GLOBAL", bytes)
}

func PublishActiveCountEvent(ctx context.Context, publisher Publisher, count int64) {
	publishCountEvent(ctx, publisher, event.ActiveCountChannel, count)
}

func PublishGameCountEvent(ctx context.Context, publisher Publisher, count int64) {
	publishCountEvent(ctx, publisher, event.GamesCountChannel, count)
}

func PublishGamesEvent(ctx context.Context, publisher Publisher, output *pb.GameOutput) {
	slog.InfoContext(ctx, "PubSub: publishing game event", "gameId", output.GameId)

	bytes, err := output.MarshalVT()
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal game message", "error", err)
		return
	}
	publisher.Publish(ctx, event.GamesChannel, output.GameId, bytes)
}

func PublishTournamentEvent(ctx context.Context, publisher Publisher, tournamentID string, payload any) {
	slog.InfoContext(ctx, "PubSub: publishing tournament event", "payload", payload)

	bytes, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal tournament event", "error", err)
		return
	}
	publisher.Publish(ctx, event.TournamentsChannel, tournamentID, bytes)
}

func PublishUserEvent(ctx context.Context, publisher Publisher, userID int64, payload any) {
	slog.InfoContext(ctx, "PubSub: publishing user event", "payload", payload)

	bytes, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal user event", "error", err)
		return
	}
	publisher.Publish(ctx, event.UsersChannel, strconv.Itoa(int(userID)), bytes)
}
