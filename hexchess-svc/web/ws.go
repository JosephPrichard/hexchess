package web

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

type message = []byte

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func writeMessage(ctx context.Context, conn *websocket.Conn, bytes []byte) {
	if bytes == nil {
		// a nil message is a "no-op", the caller does not need to check for errors
		return
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
		slog.WarnContext(ctx, "failed to write ws message", "err", err)
	}
}

func writeClose(ctx context.Context, conn *websocket.Conn, errCode string) {
	if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, errCode)); err != nil {
		slog.WarnContext(ctx, "failed to write ws close message", "err", err)
	}
}
