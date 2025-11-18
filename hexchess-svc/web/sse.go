package web

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/data"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type SseHandler = func(state *ServerState, w http.ResponseWriter, r *http.Request, f http.Flusher) error

func makeSseHandler(state *ServerState, h SseHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "received sse request", "method", r.Method, "url", r.URL)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is unsupported", http.StatusInternalServerError)
			return
		}
		if err := h(state, w, r, f); err != nil {
			slog.ErrorContext(r.Context(), "sse request failed", "method", r.Method, "url", r.URL, "error", err)
			http.Error(w, fmt.Sprintf("%s: internal server error", MetaEvent), http.StatusInternalServerError)
			f.Flush()
		}
		slog.InfoContext(r.Context(), "finished sse request", "method", r.Method, "url", r.URL)
	})
}

const MetaEvent = "meta"
const UserChallengeEvent = "userEvents"
const GamesCountEvent = "gameCountEvents"
const ActiveCountEvent = "activeCountEvents"

func writeEvent(w http.ResponseWriter, e string, d string) {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e, d); err != nil {
		slog.Error("failed to write to sse", "err", err)
	}
}

func writeCountEvent(w http.ResponseWriter, event data.UcEvent) {
	var e string
	switch event.Kind {
	case data.UcActiveEk:
		e = ActiveCountEvent
	case data.UcGamesEk:
		e = GamesCountEvent
	}
	if e == "" {
		slog.Error("unknown count event key", "event", e)
		return
	}
	writeEvent(w, e, event.Data)
}

func makeCountEvent(count int64, sseID string) string {
	b, err := json.Marshal(data.CountEvent{Count: count, ID: sseID})
	if err != nil {
		slog.Error("failed to marshal count event", "sseID", sseID, "err", err)
	}
	return string(b)
}

func makePingTicker(ctx context.Context, state *ServerState, sseID string) chan struct{} {
	ctx = context.WithoutCancel(ctx)
	stopPingChan := make(chan struct{})
	go func() {
		pingTicker := time.NewTicker(time.Second * 60)
		for {
			select {
			case <-stopPingChan:
				slog.InfoContext(ctx, "finished ping ticker", "sseID", sseID)
				return
			case <-pingTicker.C:
				if err := data.RetainActiveUser(ctx, state.Rdb, sseID); err != nil {
					slog.ErrorContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
				}
			}
		}
	}()
	return stopPingChan
}

func HandleCountEvents(state *ServerState, w http.ResponseWriter, r *http.Request, f http.Flusher) error {
	ctx := r.Context()
	sseID := state.MakeID()
	clientGone := ctx.Done()

	gamesCount, err := data.GetChessStateCount(ctx, state.Rdb)
	if err != nil {
		return err
	}
	activeCount, err := data.AddActiveUser(ctx, state.Rdb, sseID)
	if err != nil {
		return err
	}

	writeEvent(w, MetaEvent, sseID)
	writeCountEvent(w, data.UcEvent{Kind: data.UcGamesEk, Data: makeCountEvent(gamesCount, sseID)})
	f.Flush()

	countsChan := make(chan data.UcEvent)
	state.CountsCaster.Subscribe(countsChan)

	if err := data.BroadcastActiveCount(ctx, state.Rdb, activeCount, sseID); err != nil {
		slog.ErrorContext(ctx, "failed to broadcast active count", "sseID", sseID, "err", err)
	}

	stopPingChan := makePingTicker(ctx, state, sseID)

	shutdown := func() error {
		stopPingChan <- struct{}{}
		ac, err := data.RemoveActiveUser(ctx, state.Rdb, sseID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to remove active user", "sseID", sseID, "error", err)
		}
		if err := data.BroadcastActiveCount(ctx, state.Rdb, ac, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast active count on removal", "sseID", sseID, "error", err)
		}
		return nil
	}

	go func() {
		<-clientGone
		state.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse", "sseID", sseID)
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
	for {
		select {
		case e, ok := <-countsChan:
			if !ok {
				return shutdown()
			}
			writeCountEvent(w, e)
			f.Flush()
		case <-keepAliveTicker.C:
			writeEvent(w, MetaEvent, "KeepAlive")
			f.Flush()
		}
	}
}

func HandleUserEvents(state *ServerState, w http.ResponseWriter, r *http.Request, f http.Flusher) error {
	ctx := r.Context()
	sseID := state.MakeID()
	clientGone := ctx.Done()

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	writeEvent(w, MetaEvent, sseID)
	f.Flush()

	usersChan := make(chan []byte)
	state.UsersCaster.Subscribe(strID, usersChan)

	go func() {
		<-clientGone
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
			writeEvent(w, UserChallengeEvent, string(m))
			f.Flush()
		case <-keepAliveTicker.C:
			writeEvent(w, MetaEvent, "KeepAlive")
			f.Flush()
		}
	}
}
