package web

import (
	"fmt"
	"hexchess-svc/pb"
	"net/http"
	"strconv"

	"google.golang.org/protobuf/proto"
)

func (server *Server) HandleGetMoveReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	replayID, err := strconv.Atoi(r.URL.Query().Get("replayId"))
	if err != nil {
		return OneRespError("id", ErrHttpInvalidID)
	}

	bytes, err := server.Services.GetMovesHistory(ctx, replayID)
	if err != nil {
		return err
	}

	writeBytes(w, http.StatusOK, bytes)
	// aggressive cache control because this resource does not change, but the algorithm we are using to transform it might if requirements change.
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	return nil
}

func (server *Server) HandleGetGameChats(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	gameID := r.URL.Query().Get("gameId")

	chats, err := server.Services.GetStateChats(ctx, gameID, 100)
	if err != nil {
		return fmt.Errorf("get state chats: %w", err)
	}
	bytes, err := proto.Marshal(&pb.ChatMessages{
		Chats: chats,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal chats: %w", err)
	}

	writeBytes(w, http.StatusOK, bytes)
	return nil
}
