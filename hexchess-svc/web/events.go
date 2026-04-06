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

	svc "hexchess-svc/services"
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

func (server *Server) HandleCountEvents(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx

	activeCount, err := server.Services.GetActiveCount(ctx)
	if err != nil {
		return err
	}
	gamesCount, err := server.Services.GetChessStateCount(ctx)
	if err != nil {
		return err
	}

	w.writeCountEvent(svc.UcActiveEk, activeCount)
	w.writeCountEvent(svc.UcGamesEk, gamesCount)

	countsChan := make(chan svc.UcEvent, SSEChanBufCap)
	server.Broadcasters.CountsCaster.Subscribe(countsChan)

	go func() {
		<-ctx.Done() // stop from the client, so stop the RecvLoop by unsubscribing
		server.Broadcasters.CountsCaster.Unsubscribe(countsChan)
		slog.InfoContext(ctx, "finished handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
RecvLoop:
	for {
		select {
		case e, ok := <-countsChan: // RecvLoop contuines until we unsubscribe
			if !ok {
				break RecvLoop
			}
			w.writeUcEvent(e)
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}

	return nil
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
func (server *Server) HandleActiveConn(w SSEWriter, _ *http.Request) error {
	ctx := w.ctx

	sseID := server.EntropySource.MakeID()

	count, err := server.Services.AddActiveUser(ctx, sseID)
	if err != nil {
		return err
	}
	if err := server.Services.BroadcastActiveCount(ctx, count); err != nil {
		return fmt.Errorf("broadcast active user count after adding %d: %w", count, err)
	}

	w.writeEvent(MetaEvent, sseID)

	stopTimer := every(svc.ActiveUserMaxage-time.Second, func() {
		if err := server.Services.RetainActiveUser(ctx, sseID); err != nil {
			slog.ErrorContext(ctx, "failed to retain active user", "sseID", sseID, "err", err)
		}
	})

	keepAliveTicker := time.NewTicker(time.Second * 15)
RecvLoop:
	for {
		select {
		case <-ctx.Done():
			break RecvLoop
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}

	afterCtx := context.WithoutCancel(ctx)
	stopTimer <- true

	if count, err = server.Services.RemoveActiveUser(afterCtx, sseID); err != nil {
		slog.ErrorContext(afterCtx, "failed to remove active user", "sseID", sseID, "err", err)
	}
	if err := server.Services.BroadcastActiveCount(afterCtx, count); err != nil {
		slog.ErrorContext(afterCtx, "broadcast active user count after removing", "err", err)
	}

	return nil
}

func (server *Server) HandleUserEvents(w SSEWriter, r *http.Request) error {
	ctx := w.ctx

	player, _, err := server.GetSessionPlayer(ctx, r)
	if err != nil {
		if errors.Is(err, svc.ErrSessionNotFound) {
			return ErrHttpSessionExpired
		}
		return err
	}
	strID := strconv.Itoa(int(player.ID))

	w.writeEvent(MetaEvent, strconv.FormatInt(player.ID, 10))

	usersChan := make(chan []byte, SSEChanBufCap)
	server.Broadcasters.UsersCaster.Subscribe(strID, usersChan)

	go func() {
		<-ctx.Done() // stop from the client, so stop the RecvLoop by unsubscribing
		server.Broadcasters.UsersCaster.Unsubscribe(strID, usersChan)
		slog.InfoContext(ctx, "finishing handle user events sse")
	}()

	keepAliveTicker := time.NewTicker(time.Second * 15)
RecvLoop:
	for {
		select {
		case m, ok := <-usersChan: // RecvLoop contuines until we unsubscribe
			if !ok {
				break RecvLoop
			}
			w.writeEvent(UserChallengeEvent, string(m))
		case <-keepAliveTicker.C:
			w.writeEvent(MetaEvent, "KeepAlive")
		}
	}

	return nil
}
