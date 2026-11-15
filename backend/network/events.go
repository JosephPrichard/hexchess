package network

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/pubsub/subscribers"
	svc "hexchess-svc/user"
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
	// SSEChanBufCap start dropping messages after an SSE connection is lagging behind by this many messages
	SSEChanBufCap = 10
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

	sendCountEvent(client, subscribers.GlobalActiveEvent, activeCount)
	sendCountEvent(client, subscribers.GlobalGamesEvent, gamesCount)

	subscriber := server.broadcasters.Counts.Subscribe(SSEChanBufCap)

	streamServerSentEvents(client, ListenConfig[subscribers.GlobalCastEvent]{
		OnClose: func() {
			server.broadcasters.Counts.Unsubscribe(subscriber)
			slog.InfoContext(ctx, "SSE handler: finished handle user events stream")
		},
		Mailbox:    subscriber.Mailbox(),
		MapMessage: mapGlobalEvent,
	})

	return nil
}

func mapGlobalEvent(message subscribers.GlobalCastEvent) SseEvent {
	var eventName string
	switch message.Kind {
	case subscribers.GlobalActiveEvent:
		eventName = ActiveCountEvent
	case subscribers.GlobalGamesEvent:
		eventName = GamesCountEvent
	default:
		return SseEvent{}
	}
	return SseEvent{EventName: eventName, EventData: message.Data}
}

func sendCountEvent(client *SSEClient, kind subscribers.CountEventKind, count int64) {
	bytes, err := json.Marshal(publishers.CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(client.ctx, "marshal count event", "Err", err)
	}
	client.SendEvent(mapGlobalEvent(subscribers.GlobalCastEvent{Kind: kind, Data: string(bytes)}))
}

const RetainActiveUserPeriod = svc.ActiveUserMaxAge - time.Second

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

	blockSSEClient(client)
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

	subscriber := server.broadcasters.Users.Subscribe(strUserID, SSEChanBufCap)

	streamServerSentEvents(client, ListenConfig[[]byte]{
		OnClose: func() {
			server.broadcasters.Users.Unsubscribe(strUserID, subscriber)
			slog.InfoContext(ctx, "SSE handler: finishing handle user events stream")

			detachedCtx := context.WithoutCancel(ctx)
			if _, err = server.service.RemoveActiveUser(detachedCtx, strUserID); err != nil {
				slog.ErrorContext(detachedCtx, "SSE handler: failed to remove active user", "sseID", strUserID, "error", err)
			}
		},
		Mailbox: subscriber.Mailbox(),
		MapMessage: func(message []byte) SseEvent {
			return SseEvent{EventName: UserChallengeEvent, EventData: string(message)}
		},
	})

	return nil
}

func (server *HttpServer) HandleTournamentEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	subscriber := server.broadcasters.Tournament.Subscribe(tournamentKey, SSEChanBufCap)

	client.SendEvent(SseEvent{EventName: MetaEvent, EventData: tournamentKey})

	streamServerSentEvents(client, ListenConfig[[]byte]{
		OnClose: func() {
			server.broadcasters.Tournament.Unsubscribe(tournamentKey, subscriber)
			slog.InfoContext(ctx, "SSE handler: finishing handle tournament events stream")
		},
		Mailbox: subscriber.Mailbox(),
		MapMessage: func(message []byte) SseEvent {
			return SseEvent{EventName: TournamentEvent, EventData: string(message)}
		},
	})

	return nil
}
