package network

import (
	"context"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service/user"
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

func (server *Server) HandleCountEvents(client *SSEClient, _ *http.Request) error {
	ctx := client.ctx

	activeCount, err := server.services.GetActiveCount(ctx)
	if err != nil {
		return serrors.New("get active count", err)
	}
	gamesCount, err := server.services.GetGameMetadataCount(ctx)
	if err != nil {
		return serrors.New("get chess state count", err)
	}

	writeCountEvent(client, pubsub.GlobalActiveEvent, activeCount)
	writeCountEvent(client, pubsub.GlobalGamesEvent, gamesCount)

	countsChan := make(chan pubsub.GlobalCastEvent, SSEChanBufCap)
	server.broadcasters.Counts.Subscribe(countsChan)

	go func() {
		<-ctx.Done()
		server.broadcasters.Counts.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events stream")
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case event, ok := <-countsChan:
			if !ok {
				return nil
			}
			writeGlobalEvent(client, event)
		case <-keepAliveTicker.C:
			client.event(MetaEvent, "KeepAlive")
		}
	}
}

const RetainActiveUserPeriod = svc.ActiveUserMaxAge - time.Second

// HandleActiveConn is a long-lived TCP connection used to maintain an active user, it only ever receives "meta" messages
func (server *Server) HandleActiveConn(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	if _, err := server.services.AddActiveUser(ctx, strUserID); err != nil {
		return serrors.New("add active user", err)
	}

	client.event(MetaEvent, strUserID)

	stop := timeutil.Schedule(RetainActiveUserPeriod, func() {
		if err := server.services.RetainActiveUser(ctx, strUserID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "userID", strUserID, "error", err)
		}
	})
	defer stop()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-keepAliveTicker.C:
			client.event(MetaEvent, "KeepAlive")
		}
	}
}

func (server *Server) HandleUserEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := server.authenticator.GetSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	client.event(MetaEvent, strUserID)

	usersChan := make(chan []byte, SSEChanBufCap)
	server.broadcasters.Users.Subscribe(strUserID, usersChan)

	go func() {
		<-ctx.Done()
		server.broadcasters.Users.Unsubscribe(strUserID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events stream")

		detachedCtx := context.WithoutCancel(ctx)
		if _, err = server.services.RemoveActiveUser(detachedCtx, strUserID); err != nil {
			slog.ErrorContext(detachedCtx, "failed to remove active user", "sseID", strUserID, "error", err)
		}
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case message, ok := <-usersChan:
			if !ok {
				return nil
			}
			client.event(UserChallengeEvent, string(message))
		case <-keepAliveTicker.C:
			client.event(MetaEvent, "KeepAlive")
		}
	}
}

func (server *Server) HandleTournamentEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	tournamentChan := make(chan []byte, SSEChanBufCap)
	server.broadcasters.Tournament.Subscribe(tournamentKey, tournamentChan)

	client.event(MetaEvent, tournamentKey)

	go func() {
		<-ctx.Done()
		server.broadcasters.Tournament.Unsubscribe(tournamentKey, tournamentChan)
		slog.InfoContext(ctx, "finishing handle tournament events stream")
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case message, ok := <-tournamentChan:
			if !ok {
				return nil
			}
			client.event(TournamentEvent, string(message))
		case <-keepAliveTicker.C:
			client.event(MetaEvent, "KeepAlive")
		}
	}
}
