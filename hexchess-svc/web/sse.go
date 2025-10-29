package web

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/data"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type SseHandler = func(w http.ResponseWriter, r *http.Request, f http.Flusher, state ServerState) error

func makeSseHandler(state ServerState, h SseHandler) http.Handler {
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
		if err := h(w, r, f, state); err != nil {
			slog.ErrorContext(r.Context(), "sse request failed", "method", r.Method, "url", r.URL, "error", err)
			http.Error(w, fmt.Sprintf("%s: internal server error", MetaEvent), http.StatusInternalServerError)
			f.Flush()
		}
	})
}

func writeEvent(w http.ResponseWriter, key string, value string) {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", key, value)
	if err != nil {
		slog.ErrorContext(context.Background(), "failed to write to sse", "err", err)
	}
}

const MetaEvent = "meta"
const UserChallengeEvent = "user-challenge"
const GamesCountEvent = "gameCountEvents"
const ActiveCountEvent = "userCountEvents"

func shutdownActiveUser(ctx context.Context, state ServerState, sseID string) {
	// a user will get expired, but remove it and broadcast the new count to keep all users up to date
	ac, err := data.RemoveActiveUser(ctx, state.Rdb, sseID)
	if err == nil {
		if err := data.BroadcastActiveCount(ctx, state.Rdb, ac); err != nil {
			slog.ErrorContext(ctx, "failed to broadcast active count", "sseID", sseID, "err", err)
		}
	} else {
		slog.ErrorContext(ctx, "failed to remove active user", "sseID", sseID, "err", err)
	}
}

func HandleCountEvents(w http.ResponseWriter, r *http.Request, f http.Flusher, state ServerState) error {
	ctx := r.Context()

	sseID := uuid.NewString()
	clientGone := ctx.Done()

	sc, err := data.GetChessStateCount(ctx, state.Rdb)
	if err != nil {
		return err
	}
	ac, err := data.AddActiveUser(ctx, state.Rdb, sseID)
	if err != nil {
		return err
	}
	if err := data.BroadcastActiveCount(ctx, state.Rdb, ac); err != nil {
		return err
	}

	writeEvent(w, MetaEvent, "Connected")
	writeEvent(w, ActiveCountEvent, strconv.FormatInt(ac, 10))
	writeEvent(w, GamesCountEvent, strconv.FormatInt(sc, 10))
	f.Flush()

	activeChan := make(chan []byte)
	state.ActiveCntCaster.Subscribe(activeChan)
	defer state.ActiveCntCaster.Unsubscribe(activeChan)

	gamesChan := make(chan []byte)
	state.GamesCntCaster.Subscribe(gamesChan)
	defer state.GamesCntCaster.Unsubscribe(gamesChan)

	var sseMu sync.Mutex
	writeEvent := func(key string, value string) {
		sseMu.Lock()
		defer sseMu.Unlock()
		writeEvent(w, key, value)
		f.Flush()
	}

	go func() {
		for msg := range activeChan {
			writeEvent(ActiveCountEvent, string(msg))
		}
	}()
	go func() {
		for msg := range gamesChan {
			writeEvent(GamesCountEvent, string(msg))
		}
	}()
	for {
		pingTicker := time.NewTimer(time.Minute)
		keepAliveTicker := time.NewTicker(time.Second * 15)
		select {
		case <-clientGone:
			shutdownActiveUser(ctx, state, sseID)
			slog.InfoContext(ctx, "finishing handle count events sse", "sseID", sseID)
			return nil
		case <-keepAliveTicker.C:
			writeEvent(MetaEvent, "KeepAlive")
		case <-pingTicker.C:
			if err := data.RetainActiveUser(ctx, state.Rdb, sseID); err != nil {
				slog.ErrorContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
			}
		}
	}
}

func HandleUserEvents(w http.ResponseWriter, r *http.Request, f http.Flusher, state ServerState) error {
	ctx := r.Context()
	sseID := uuid.NewString()
	clientGone := ctx.Done()

	player, _, err := GetSessionPlayer(ctx, state.Rdb, r)
	if err != nil {
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	writeEvent(w, MetaEvent, "Connected")
	f.Flush()

	usersChan := make(chan []byte)
	state.UsersCaster.Subscribe(strID, usersChan)

	for {
		keepAliveTicker := time.NewTicker(time.Second * 15)
		select {
		case m := <-usersChan:
			writeEvent(w, UserChallengeEvent, string(m))
			f.Flush()
		case <-clientGone:
			state.UsersCaster.Unsubscribe(strID, usersChan)
			slog.InfoContext(ctx, "finishing handle user events sse", "sseID", sseID)
			return nil
		case <-keepAliveTicker.C:
			writeEvent(w, MetaEvent, "KeepAlive")
			f.Flush()
		}
	}
}
