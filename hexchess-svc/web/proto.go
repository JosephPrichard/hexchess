package web

import (
	"net/http"
	"strconv"
)

func (server *Server) HandleGetMoveReplay(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	replayID, err := strconv.Atoi(r.URL.Query().Get("replayId"))
	if err != nil {
		return OneRespError("id", ErrHttpInvalidID)
	}

	bResp, err := server.Services.GetMovesHistory(ctx, replayID)
	if err != nil {
		return err
	}

	writeBytes(w, http.StatusOK, bResp)
	// aggressive cache control because this resource does not change, but the algorithm we are using to transform it might if requirements change.
	//w.Header().Set("Cache-Control", "public, max-age=3600")
	return nil
}
