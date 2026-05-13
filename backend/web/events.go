package web

import (
	"context"
	"fmt"
	"hexchess-svc/internal/timeutil"
	"hexchess-svc/pubsub"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	svc "hexchess-svc/service"
)

func SSE(h func(w *SSEClient, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "received sse request", "method", r.Method, "url", r.URL)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is unsupported", http.StatusInternalServerError)
			return
		}
		ctx := r.Context()
		if err := h(&SSEClient{ctx, w, f}, r); err != nil {
			status, m := HttpStatusFromErr(err)

			slog.Log(ctx, LevelFromStatus(status), "sse request failed", "error", err, "method", r.Method, "url", r.URL)

			http.Error(w, fmt.Sprintf("%s:%s", MetaEvent, m), status)
		}
		slog.InfoContext(ctx, "finished sse request", "method", r.Method, "url", r.URL)
	}
}

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

func (api *API) HandleCountEvents(client *SSEClient, _ *http.Request) error {
	ctx := client.ctx

	activeCount, err := api.services.GetActiveCount(ctx)
	if err != nil {
		return fmt.Errorf("get active count: %w", err)
	}
	gamesCount, err := api.services.GetChessStateCount(ctx)
	if err != nil {
		return fmt.Errorf("get chess state count: %w", err)
	}

	writeCountEvent(client, pubsub.GlobalActiveEvent, activeCount)
	writeCountEvent(client, pubsub.GlobalGamesEvent, gamesCount)

	countsChan := make(chan pubsub.GlobalCastEvent, SSEChanBufCap)
	api.broadcasters.CountsCaster.Subscribe(countsChan)

	go func() {
		<-ctx.Done()
		api.broadcasters.CountsCaster.Unsubscribe(countsChan)
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

const RetainActiveUserPeriod = svc.ActiveUserMaxage - time.Second

// HandleActiveConn is a long-lived TCP connection used to maintain an active user, it only ever receives "meta" messages
func (api *API) HandleActiveConn(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := api.authenticator.ExpectSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	if _, err := api.services.AddActiveUser(ctx, strUserID); err != nil {
		return fmt.Errorf("add active user: %w", err)
	}

	client.event(MetaEvent, strUserID)

	stop := timeutil.Every(RetainActiveUserPeriod, func() {
		if err := api.services.RetainActiveUser(ctx, strUserID); err != nil {
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

func (api *API) HandleUserEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	player, err := api.authenticator.ExpectSessionPlayer(ctx, r)
	if err != nil {
		return err
	}
	strUserID := strconv.Itoa(int(player.ID))

	client.event(MetaEvent, strUserID)

	usersChan := make(chan []byte, SSEChanBufCap)
	api.broadcasters.UsersCaster.Subscribe(strUserID, usersChan)

	go func() {
		<-ctx.Done()
		api.broadcasters.UsersCaster.Unsubscribe(strUserID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events stream")

		detatchedCtx := context.WithoutCancel(ctx)
		if _, err = api.services.RemoveActiveUser(detatchedCtx, strUserID); err != nil {
			slog.ErrorContext(detatchedCtx, "failed to remove active user", "sseID", strUserID, "error", err)
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

func (api *API) HandleTournamentEvents(client *SSEClient, r *http.Request) error {
	ctx := client.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	tournamentChan := make(chan []byte, SSEChanBufCap)
	api.broadcasters.TournamentCaster.Subscribe(tournamentKey, tournamentChan)

	client.event(MetaEvent, tournamentKey)

	go func() {
		<-ctx.Done()
		api.broadcasters.TournamentCaster.Unsubscribe(tournamentKey, tournamentChan)
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
