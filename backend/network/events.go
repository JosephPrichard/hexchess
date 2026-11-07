package network

import (
	"context"
	"encoding/json"
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

	countsChan := make(chan subscribers.GlobalCastEvent, SSEChanBufCap)
	server.broadcasters.Counts.Subscribe(countsChan)

	client.StreamMessages(ListenConfig[subscribers.GlobalCastEvent]{
		Stream: countsChan,
		OnClose: func() {
			server.broadcasters.Counts.Unsubscribe(countsChan)
			slog.InfoContext(ctx, "finished handle user events stream")
		},
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
			slog.ErrorContext(ctx, "failed to retain active user", "userID", strUserID, "error", err)
		}
	})
	defer stopScheduledEvent()

	client.Block()
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

	usersChan := make(chan []byte, SSEChanBufCap)
	server.broadcasters.Users.Subscribe(strUserID, usersChan)

	client.StreamMessages(ListenConfig[[]byte]{
		Stream: usersChan,
		OnClose: func() {
			server.broadcasters.Users.Unsubscribe(strUserID, usersChan)
			slog.InfoContext(ctx, "finishing handle user events stream")

			detachedCtx := context.WithoutCancel(ctx)
			if _, err = server.service.RemoveActiveUser(detachedCtx, strUserID); err != nil {
				slog.ErrorContext(detachedCtx, "failed to remove active user", "sseID", strUserID, "error", err)
			}
		},
		MapMessage: func(message []byte) SseEvent {
			return SseEvent{EventName: UserChallengeEvent, EventData: string(message)}
		},
	})

	return nil
}

func (server *HttpServer) HandleTournamentEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	tournamentChan := make(chan []byte, SSEChanBufCap)
	server.broadcasters.Tournament.Subscribe(tournamentKey, tournamentChan)

	client.SendEvent(SseEvent{EventName: MetaEvent, EventData: tournamentKey})

	client.StreamMessages(ListenConfig[[]byte]{
		Stream: tournamentChan,
		OnClose: func() {
			server.broadcasters.Tournament.Unsubscribe(tournamentKey, tournamentChan)
			slog.InfoContext(ctx, "finishing handle tournament events stream")
		},
		MapMessage: func(message []byte) SseEvent {
			return SseEvent{EventName: TournamentEvent, EventData: string(message)}
		},
	})

	return nil
}
