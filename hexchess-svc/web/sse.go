package web

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type SseHandler = func(w http.ResponseWriter, r *http.Request, f http.Flusher, state ServerState) error

func makeSseHandler(state ServerState, h SseHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := uuid.NewString()
		r = r.WithContext(context.WithValue(r.Context(), lib.TraceKey, trace))

		slog.InfoContext(r.Context(), "received sse request", "method", r.Method, "url", r.URL)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

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

const MetaEvent = "meta"
const UserChallengeEvent = "user-challenge"
const GamesCountEvent = "gameCountEvents"
const ActiveCountEvent = "userCountEvents"

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

	fmt.Fprintf(w, "%s: %s\n", MetaEvent, "Connected")
	fmt.Fprintf(w, "%s: %s\n", ActiveCountEvent, strconv.FormatInt(ac, 10))
	fmt.Fprintf(w, "%s: %s\n", GamesCountEvent, strconv.FormatInt(sc, 10))
	f.Flush()

	activeChan := make(chan []byte)
	state.ActiveCaster.Subscribe(activeChan)
	gamesChan := make(chan []byte)
	state.GamesCaster.Subscribe(gamesChan)

	type event struct {
		key   string
		value []byte
	}

	eventChan := make(chan event)

	go func() {
		for msg := range activeChan {
			eventChan <- event{key: ActiveCountEvent, value: msg}
		}
	}()
	go func() {
		for msg := range gamesChan {
			eventChan <- event{key: GamesCountEvent, value: msg}
		}
	}()
	for {
		pingTicker := time.NewTimer(time.Minute)
		keepAliveTicker := time.NewTicker(time.Second * 15)
		select {
		case m := <-eventChan:
			fmt.Fprintf(w, "%s: %s\n", m.key, m.value)
			f.Flush()
		case <-clientGone:
			state.ActiveCaster.Unsubscribe(activeChan)
			state.GamesCaster.Unsubscribe(gamesChan)
			slog.InfoContext(ctx, "finishing handle count events sse", "sseID", sseID)
			return nil
		case <-keepAliveTicker.C:
			fmt.Fprintf(w, "%s: %s\n", MetaEvent, "KeepAlive")
			f.Flush()
		case <-pingTicker.C:
			if err := data.RetainActiveUser(ctx, state.Rdb, sseID); err != nil {
				slog.InfoContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
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

	fmt.Fprintf(w, "%s: %s\n", MetaEvent, "Connected")
	f.Flush()

	usersChan := make(chan []byte)
	state.UsersCaster.Subscribe(strID, usersChan)

	for {
		keepAliveTicker := time.NewTicker(time.Second * 15)
		select {
		case m := <-usersChan:
			fmt.Fprintf(w, "%s: %s\n", UserChallengeEvent, string(m))
			f.Flush()
		case <-clientGone:
			state.UsersCaster.Unsubscribe(strID, usersChan)
			slog.InfoContext(ctx, "finishing handle user events sse", "sseID", sseID)
			return nil
		case <-keepAliveTicker.C:
			fmt.Fprintf(w, "%s: %s\n", MetaEvent, "KeepAlive")
			f.Flush()
		}
	}
}
