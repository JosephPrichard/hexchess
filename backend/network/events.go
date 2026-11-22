package network

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/user"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/timeutil"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	KeepAliveTimeout   = time.Second * 15
	MetaEvent          = "meta"
	UserChallengeEvent = "userEvents"
	GamesCountEvent    = "gameCountEvents"
	ActiveCountEvent   = "activeCountEvents"
	TournamentEvent    = "tournamentEvent"
)

func (server *HttpServer) HandleCountEvents(client *SSEClient, _ *http.Request) error {
	ctx := client.ctx

	activeCount, err := server.service.GetActiveCount(ctx)
	if err != nil {
		return serrors.New("get active count", err)
	}
	gamesCount, err := server.service.GetGameMetadataCount(ctx)
	if err != nil {
		return serrors.New("get chess state count", err)
	}

	sendCountEvent(client, event.ActiveCountEvent, activeCount)
	sendCountEvent(client, event.GamesCountEvent, gamesCount)

	subjects := []pubsub.Subject{
		{Channel: event.ActiveCountChannel, Topic: "GLOBAL"},
		{Channel: event.GamesCountChannel, Topic: "GLOBAL"},
	}
	subscriber, unsubscribe, err := pubsub.SubscribeMany(ctx, server.pubsub, subjects)
	if err != nil {
		return serrors.New("subscribe to pubsub broker", err)
	}
	defer unsubscribe()

	channelToSSE(client, subscriber, mapCountEvent)

	slog.InfoContext(ctx, "SSE handler: finished handle user events stream")
	return nil
}

func mapCountEvent(payload []byte) SseEvent {
	var message event.CountEvent
	if err := json.Unmarshal(payload, &message); err != nil {
		slog.Error("failed to unmarshal global event", "error", err)
		return SseEvent{}
	}

	var eventName string
	switch message.Kind {
	case event.ActiveCountEvent:
		eventName = ActiveCountEvent
	case event.GamesCountEvent:
		eventName = GamesCountEvent
	default:
		return SseEvent{}
	}

	return SseEvent{EventName: eventName, EventData: string(payload)}
}

const RetainActiveUserPeriod = user.ActiveUserMaxAge - time.Second

// HandleActiveConn is a long-lived TCP connection used to maintain an active user, it only ever receives "meta" messages
func (server *HttpServer) HandleActiveConn(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	if _, err := server.service.AddActiveUser(ctx, strUserID); err != nil {
		return serrors.New("add active user", err)
	}

	client.SendEvent(SseEvent{EventName: MetaEvent, EventData: strUserID})

	stopScheduledEvent := timeutil.Schedule(RetainActiveUserPeriod, func() {
		if err := server.service.RetainActiveUser(ctx, strUserID); err != nil {
			slog.ErrorContext(ctx, "SSE handler: failed to retain active user", "userID", strUserID, "error", err)
		}
	})
	defer stopScheduledEvent()

	blockSSE(client)
	return nil
}

func (server *HttpServer) HandleUserEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	client.SendEvent(SseEvent{EventName: MetaEvent, EventData: strUserID})

	subscriber, unsubscribe, err := server.pubsub.Subscribe(ctx, event.UsersChannel, strUserID)
	if err != nil {
		return serrors.New("subscribe to pubsub broker", err)
	}
	defer unsubscribe()

	channelToSSE(client, subscriber, func(message []byte) SseEvent {
		return SseEvent{EventName: UserChallengeEvent, EventData: string(message)}
	})

	detachedCtx := context.WithoutCancel(ctx)
	if _, err = server.service.RemoveActiveUser(detachedCtx, strUserID); err != nil {
		slog.ErrorContext(detachedCtx, "SSE handler: failed to remove active user", "sseID", strUserID, "error", err)
	}

	slog.InfoContext(ctx, "SSE handler: finishing handle user events stream")
	return nil
}

func (server *HttpServer) HandleTournamentEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	subscriber, unsubscribe, err := server.pubsub.Subscribe(ctx, event.TournamentsChannel, tournamentKey)
	if err != nil {
		return serrors.New("subscribe to pubsub broker", err)
	}
	defer unsubscribe()

	client.SendEvent(SseEvent{EventName: MetaEvent, EventData: tournamentKey})

	channelToSSE(client, subscriber, func(message []byte) SseEvent {
		return SseEvent{EventName: TournamentEvent, EventData: string(message)}
	})

	slog.InfoContext(ctx, "SSE handler: finishing handle tournament events stream")
	return nil
}
