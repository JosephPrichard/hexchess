package network

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type wsMessage = []byte

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
	conn.SetWriteDeadline(time.Now().Add(time.Second * 1))
	if err := conn.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
		slog.WarnContext(ctx, "failed to write ws message", "error", err)
	}
}
