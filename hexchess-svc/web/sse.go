package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/timeutil"
	"hexchess-svc/services"
	"log/slog"
	"net/http"
	"strconv"
	"time"
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

type SSEHandler struct {
	Rdb *db.Rdb
	svc.LocalBroadcasters
	outbound.Generator
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
	sse.f.Flush()
}

func (sse SSEWriter) writeUcEvent(event svc.UcEvent) {
	var e string
	switch event.Kind {
	case svc.UcActiveEk:
		e = ActiveCountEvent
	case svc.UcGamesEk:
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
	MetaEvent          = "meta"
	UserChallengeEvent = "userEvents"
	GamesCountEvent    = "gameCountEvents"
	ActiveCountEvent   = "activeCountEvents"
	// SSEChanBufCap start dropping messages after an SSE connection is lagging behind by this many messages
	SSEChanBufCap = 10
)

func (h *SSEHandler) HandleCountEvents(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx

	activeCount, err := svc.MakeActiveScenario().GetActiveCount(ctx, h.Rdb)
	if err != nil {
		return err
	}
	gamesCount, err := svc.GetChessStateCount(ctx, h.Rdb)
	if err != nil {
		return err
	}

	w.writeCountEvent(svc.UcActiveEk, activeCount)
	w.writeCountEvent(svc.UcGamesEk, gamesCount)

	countsChan := make(chan svc.UcEvent, SSEChanBufCap)
	h.CountsCaster.Subscribe(countsChan)

	go func() {
		<-ctx.Done()
		h.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
	for {
		select {
		case e, ok := <-countsChan:
			if !ok {
				return nil
			}
			w.writeUcEvent(e)
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}

// HandleActiveCountConn a long-lived TCP connection used to maintain an active connection, it only ever receives "meta" messages
func (h *SSEHandler) HandleActiveCountConn(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx
	scenario := svc.MakeActiveScenario()

	sseID := h.MakeID()

	count, err := scenario.AddActiveUser(ctx, h.Rdb, sseID)
	if err != nil {
		return err
	}
	if err := svc.BroadcastActiveCount(ctx, h.Rdb, count); err != nil {
		return fmt.Errorf("broadcast active user count after adding %d: %w", count, err)
	}

	stopTimer := timeutil.Every(time.Second*15, func() bool {
		if err := scenario.RetainActiveUser(ctx, h.Rdb, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
		}
		w.writeEvent(MetaEvent, "KeepAlive")
		return true
	})

	w.writeEvent(MetaEvent, sseID)

	shtdwnCtx := context.WithoutCancel(ctx)
	<-ctx.Done()
	stopTimer <- true

	if count, err = scenario.RemoveActiveUser(shtdwnCtx, h.Rdb, sseID); err != nil {
		slog.ErrorContext(shtdwnCtx, "failed to remove active user", "sseID", sseID, "err", err)
	}
	if err := svc.BroadcastActiveCount(shtdwnCtx, h.Rdb, count); err != nil {
		slog.ErrorContext(shtdwnCtx, "broadcast active user count after removing", "err", err)
	}

	return nil
}

func (h *SSEHandler) HandleUserEvents(w SSEWriter, r *http.Request) error {
	ctx := w.ctx

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if err != nil {
		if errors.Is(err, svc.ErrSessionNotFound) {
			return ErrHttpSessionExpired
		}
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	w.writeEvent(MetaEvent, strconv.FormatInt(player.ID, 10))

	usersChan := make(chan []byte, SSEChanBufCap)
	h.UsersCaster.Subscribe(strID, usersChan)

	go func() {
		<-ctx.Done()
		h.UsersCaster.Unsubscribe(strID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
	for {
		select {
		case m, ok := <-usersChan:
			if !ok {
				return nil
			}
			w.writeEvent(UserChallengeEvent, string(m))
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}
