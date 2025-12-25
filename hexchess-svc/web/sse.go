package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/logutil"
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
	Rdb *db.Redis
	outbound.Generators
	svc.Broadcasters
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

func (sse SSEWriter) writeCountEvent(event svc.UcEvent) {
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

func (sse SSEWriter) writeGamesCountEvent(count int64, sseID string) {
	b, err := json.Marshal(svc.CountEvent{Count: count, ID: sseID})
	if err != nil {
		slog.ErrorContext(sse.ctx, "marshal count event", "sseID", sseID, "err", err)
	}
	sse.writeCountEvent(svc.UcEvent{Kind: svc.UcGamesEk, Data: string(b)})
}

const MetaEvent = "meta"
const UserChallengeEvent = "userEvents"
const GamesCountEvent = "gameCountEvents"
const ActiveCountEvent = "activeCountEvents"

func (h *SSEHandler) HandleCountEvents(w SSEWriter, r *http.Request) error {
	sseID := h.MakeID()
	ctx := context.WithValue(r.Context(), logutil.SseID, sseID)
	w.ctx = ctx

	gamesCount, err := svc.GetChessStateCount(ctx, h.Rdb)
	if err != nil {
		return err
	}
	activeCount, err := svc.AddActiveUser(ctx, h.Rdb, sseID)
	if err != nil {
		return err
	}

	w.writeEvent(MetaEvent, sseID)
	w.writeGamesCountEvent(gamesCount, sseID)

	countsChan := make(chan svc.UcEvent)
	h.CountsCaster.Subscribe(countsChan)

	if err := svc.BroadcastActiveCount(ctx, h.Rdb, activeCount, sseID); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast active count", "sseID", sseID, "err", err)
	}

	stopPing := timeutil.Every(time.Minute, func() bool {
		if err := svc.RetainActiveUser(ctx, h.Rdb, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "err", err)
		}
		return true
	})

	go func() {
		<-ctx.Done()
		stopPing <- true
		h.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse", "sseID", sseID)

		// shutdown
		ctx := context.WithoutCancel(ctx)
		ac, err := svc.RemoveActiveUser(ctx, h.Rdb, sseID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to remove active user", "sseID", sseID, "err", err)
		}
		if err := svc.BroadcastActiveCount(ctx, h.Rdb, ac, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast active count on removal", "sseID", sseID, "err", err)
		}
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
	for {
		select {
		case e, ok := <-countsChan:
			if !ok {
				return nil
			}
			w.writeCountEvent(e)
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}
}

func (h *SSEHandler) HandleUserEvents(w SSEWriter, r *http.Request) error {
	sseID := h.MakeID()
	ctx := context.WithValue(r.Context(), logutil.SseID, sseID)
	w.ctx = ctx

	player, _, err := GetSessionPlayer(ctx, h.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return ErrHttpSessionExpired
	}
	if err != nil {
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	w.writeEvent(MetaEvent, sseID)

	usersChan := make(chan []byte)
	h.UsersCaster.Subscribe(strID, usersChan)

	go func() {
		<-ctx.Done()
		h.UsersCaster.Unsubscribe(strID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events sse", "sseID", sseID)
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
