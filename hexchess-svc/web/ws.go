package web

import (
	"context"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func writeConn(ctx context.Context, conn *websocket.Conn, bytes []byte) {
	if bytes == nil {
		// a nil message is a "no-op", the caller does not need to check for errors
		return
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
		slog.WarnContext(ctx, "failed to write ws message", "err", err)
	}
}
