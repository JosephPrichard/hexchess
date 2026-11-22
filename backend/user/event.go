package user

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/challenges"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/event"
	"log/slog"
	"strconv"
)

type UserMessageKind string

const (
	EventChallengeKind UserMessageKind = "challenge"
)

type UserMessageType struct {
	Kind UserMessageKind `json:"kind"`
}

type UserEvent struct {
	Kind      UserMessageKind      `json:"kind"`
	UserID    int64                `json:"userId"`
	Challenge challenges.Challenge `json:"challenge"`
}

func PublishActiveCountEvent(ctx context.Context, publisher pubsub.Publisher, count int64) {
	bytes, err := json.Marshal(event.CountEvent{Kind: event.ActiveCountEvent, Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal count event", "error", err)
		return
	}
	publisher.Publish(ctx, event.ActiveCountChannel, "GLOBAL", bytes)
}

func PublishUserEvent(ctx context.Context, publisher pubsub.Publisher, e UserEvent) {
	slog.InfoContext(ctx, "PubSub: publishing user event", "event", e)

	switch e.Kind {
	case EventChallengeKind:
		e.UserID = e.Challenge.ChallengeeID
	// TODO(Joseph): add other user event cases here
	default:
		slog.WarnContext(ctx, "PubSub: unknown user event kind - skipping", "kind", e.Kind)
		return
	}

	bytes, err := json.Marshal(e)
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal user event", "error", err)
		return
	}
	publisher.Publish(ctx, event.UsersChannel, strconv.Itoa(int(e.UserID)), bytes)
}
