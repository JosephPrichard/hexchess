package web

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/testutil"
	svc "hexchess-svc/services"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetMoveReplay(t *testing.T) {
	t.Parallel()

	// given
	services := svc.SetupServicesTest(t, itest.RWPostgres)
	defer services.Close()

	wantInitialGame := chess.MakeEmptyGame(false)
	pbInitialGame := chess.SerializeGame(&wantInitialGame)

	// serialize a history that contains every field so we can check that the binary data is being stored correctly. this history doesn't actually respect game rules.
	object, err := proto.Marshal(&pb.MoveHistory{
		InitialGame: pbInitialGame,
		Steps: []*pb.HistMove{
			{Piece: 1, FromFile: 1, FromRank: 2, ToFile: 3, ToRank: 4, Notation: "pc5"},
		},
	})
	require.NoError(t, err)
	services.Query().InsertReplayMoveHistories(t.Context(), db.InsertReplayMoveHistoriesParams{
		ReplayID: 1,
		Data:     object,
	})

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/replay/move-list?replayId=%d", 1), nil)
	w := httptest.NewRecorder()

	// when
	hander := MakeServeMux(Setup{Services: services})
	hander.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	require.NoError(t, err)

	var pbMoveHist pb.MoveHistory
	require.NoError(t, proto.Unmarshal(body, &pbMoveHist))

	// then
	wantMoveReplay := &pb.MoveHistory{
		InitialGame: pbInitialGame,
		Steps: []*pb.HistMove{
			{Piece: 1, FromFile: 1, FromRank: 2, ToFile: 3, ToRank: 4, Notation: "pc5"},
		},
	}
	assert.Equal(t, http.StatusOK, w.Code)
	testutil.Equal(t, wantMoveReplay, &pbMoveHist, protocmp.Transform())
}
