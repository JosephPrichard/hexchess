package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/services"
	"hexchess-svc/util"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type SseHandler = func(state *ServerState, w SSEWriter, r *http.Request) error

func MakeSseHandler(state *ServerState, h SseHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sseID := state.MakeID()

		r = r.WithContext(context.WithValue(r.Context(), util.SseID, sseID))
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
		if err := h(state, SSEWriter{ctx, sseID, w, f}, r); err != nil {
			status, m := HttpStatusFromErr(err)
			slog.ErrorContext(ctx, "sse request failed", "err", err, "method", r.Method, "url", r.URL)
			http.Error(w, fmt.Sprintf("%s:%s", MetaEvent, m), status)
		}
		slog.InfoContext(ctx, "finished sse request", "method", r.Method, "url", r.URL)
	})
}

type SSEWriter struct {
	ctx   context.Context
	sseID string
	w     http.ResponseWriter
	f     http.Flusher
}

func (sse SSEWriter) writeEvent(e string, d string) {
	if _, err := fmt.Fprintf(sse.w, "event: %s\ndata: %s\n\n", e, d); err != nil {
		slog.ErrorContext(sse.ctx, "write to sse", "err", err, "sseID", sse.sseID)
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
		slog.ErrorContext(sse.ctx, "unknown count event key", "event", e, "sseID", sse.sseID)
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

func HandleCountEvents(state *ServerState, w SSEWriter, r *http.Request) error {
	ctx := r.Context()
	sseID := w.sseID

	gamesCount, err := svc.GetChessStateCount(ctx, state.Rdb)
	if err != nil {
		return err
	}
	activeCount, err := svc.AddActiveUser(ctx, state.Rdb, sseID)
	if err != nil {
		return err
	}

	w.writeEvent(MetaEvent, sseID)
	w.writeGamesCountEvent(gamesCount, sseID)

	countsChan := make(chan svc.UcEvent)
	state.CountsCaster.Subscribe(countsChan)

	if err := svc.BroadcastActiveCount(ctx, state.Rdb, activeCount, sseID); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast active count", "sseID", sseID, "err", err)
	}

	stopPing := util.Every(time.Minute, func() bool {
		if err := svc.RetainActiveUser(ctx, state.Rdb, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "err", err)
		}
		return true
	})

	go func() {
		<-ctx.Done()
		stopPing <- true
		state.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse", "sseID", sseID)

		// shutdown
		ctx := context.WithoutCancel(ctx)
		ac, err := svc.RemoveActiveUser(ctx, state.Rdb, sseID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to remove active user", "sseID", sseID, "err", err)
		}
		if err := svc.BroadcastActiveCount(ctx, state.Rdb, ac, sseID); err != nil {
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

func HandleUserEvents(state *ServerState, w SSEWriter, r *http.Request) error {
	ctx := r.Context()
	sseID := w.sseID

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if errors.Is(err, svc.ErrSessionNotFound) {
		return ErrHttpSessionExpired
	}
	if err != nil {
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	w.writeEvent(MetaEvent, sseID)

	usersChan := make(chan []byte)
	state.UsersCaster.Subscribe(strID, usersChan)

	go func() {
		<-ctx.Done()
		state.UsersCaster.Unsubscribe(strID, usersChan)
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
