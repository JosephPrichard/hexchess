package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	svc "hexchess-svc/service"
)

func SSE(h func(w SSEWriter, r *http.Request) error) http.HandlerFunc {
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
		if err := h(SSEWriter{ctx, w, f}, r); err != nil {
			status, m := HttpStatusFromErr(err)
			slog.ErrorContext(ctx, "sse request failed", "err", err, "method", r.Method, "url", r.URL)
			http.Error(w, fmt.Sprintf("%s:%s", MetaEvent, m), status)
		}
		slog.InfoContext(ctx, "finished sse request", "method", r.Method, "url", r.URL)
	}
}

type SSEWriter struct {
	ctx context.Context
	w   http.ResponseWriter
	f   http.Flusher
}

func (sse SSEWriter) writeEvent(e string, d string) {
	if _, err := fmt.Fprintf(sse.w, "event: %s\ndata: %s\n\n", e, d); err != nil {
		slog.ErrorContext(sse.ctx, "write to sse", "err", err)
	}

	slog.Info("writing server sent event", "event", e, "data", d)

	sse.f.Flush()
}

func (sse SSEWriter) writeUcEvent(event svc.UcEvent) {
	var e string
	switch event.Kind {
	case svc.UcActiveEvent:
		e = ActiveCountEvent
	case svc.UcGamesEvent:
		e = GamesCountEvent
	}
	if e == "" {
		slog.ErrorContext(sse.ctx, "unknown count event key", "event", e)
		return
	}
	sse.writeEvent(e, event.Data)
}

func (sse SSEWriter) writeCountEvent(kind svc.UcEventKind, count int64) {
	b, err := json.Marshal(svc.CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(sse.ctx, "marshal count event", "err", err)
	}
	sse.writeUcEvent(svc.UcEvent{Kind: kind, Data: string(b)})
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

func (api *API) HandleCountEvents(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx

	activeCount, err := api.services.GetActiveCount(ctx)
	if err != nil {
		return err
	}
	gamesCount, err := api.services.GetChessStateCount(ctx)
	if err != nil {
		return err
	}

	w.writeCountEvent(svc.UcActiveEvent, activeCount)
	w.writeCountEvent(svc.UcGamesEvent, gamesCount)

	countsChan := make(chan svc.UcEvent, SSEChanBufCap)
	api.broadcasters.CountsCaster.Subscribe(countsChan)

	go func() {
		<-ctx.Done()
		api.broadcasters.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case event, ok := <-countsChan:
			if !ok {
				return nil
			}
			w.writeUcEvent(event)
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}

func every(duration time.Duration, work func()) chan bool {
	ticker := time.NewTicker(duration)
	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-ticker.C:
				work()
			case <-stop:
				return
			}
		}
	}()

	return stop
}

// HandleActiveConn a long-lived TCP connection used to maintain an active connection, it only ever receives "meta" messages
func (api *API) HandleActiveConn(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx

	sseID := api.entropy.MakeUUID()

	count, err := api.services.AddActiveUser(ctx, sseID)
	if err != nil {
		return err
	}
	if err := api.services.BroadcastActiveCount(ctx, count); err != nil {
		return fmt.Errorf("broadcast active user count after adding %d: %w", count, err)
	}

	w.writeEvent(MetaEvent, sseID)

	stopTimer := every(svc.ActiveUserMaxage-time.Second, func() {
		if err := api.services.RetainActiveUser(ctx, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
		}
	})
	defer func() {
		stopTimer <- true
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
RecvLoop:
	for {
		select {
		case <-ctx.Done():
			break RecvLoop
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}

	detatchedCtx := context.WithoutCancel(ctx)

	if count, err = api.services.RemoveActiveUser(detatchedCtx, sseID); err != nil {
		slog.ErrorContext(detatchedCtx, "failed to remove active user", "sseID", sseID, "err", err)
	}
	if err := api.services.BroadcastActiveCount(detatchedCtx, count); err != nil {
		slog.ErrorContext(detatchedCtx, "broadcast active user count after removing", "err", err)
	}

	return nil
}

func (api *API) HandleUserEvents(w SSEWriter, r *http.Request) error {
	ctx := w.ctx

	player, _, err := api.authenticator.GetSessionPlayerAndID(ctx, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return ErrHttpSessionExpired
	} else if err != nil {
		return err
	}

	strUserID := strconv.Itoa(int(player.ID))

	w.writeEvent(MetaEvent, strUserID)

	usersChan := make(chan []byte, SSEChanBufCap)
	api.broadcasters.UsersCaster.Subscribe(strUserID, usersChan)

	go func() {
		<-ctx.Done() // stop from the client, so stop the RecvLoop by unsubscribing
		api.broadcasters.UsersCaster.Unsubscribe(strUserID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case message, ok := <-usersChan: // RecvLoop contuines until we unsubscribe
			if !ok {
				return nil
			}
			w.writeEvent(UserChallengeEvent, string(message))
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}

func (api *API) HandleTournamentEvents(w SSEWriter, r *http.Request) error {
	ctx := w.ctx

	tournamentKey := r.URL.Query().Get("tournamentKey")

	tournamentChan := make(chan []byte, SSEChanBufCap)
	api.broadcasters.TournamentCaster.Subscribe(tournamentKey, tournamentChan)

	w.writeEvent(MetaEvent, tournamentKey)

	go func() {
		<-ctx.Done() // stop from the client, so stop the RecvLoop by unsubscribing
		api.broadcasters.TournamentCaster.Unsubscribe(tournamentKey, tournamentChan)
		slog.InfoContext(ctx, "finishing handle tournament events sse")
	}()

	keepAliveTicker := time.NewTicker(KeepAliveTimeout)
	for {
		select {
		case message, ok := <-tournamentChan: // RecvLoop contuines until we unsubscribe
			if !ok {
				return nil
			}
			w.writeEvent(TournamentEvent, string(message))
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}
