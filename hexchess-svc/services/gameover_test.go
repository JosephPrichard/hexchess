package svc

import (
	"context"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"
	"math"
	"sync"
	"testing"
	"time"
)

type FakeGameHandler struct {
	lock           sync.Mutex
	cancel         func()
	outputEvents   []FinishGameEvent
	wantEventCount int
}

func (h *FakeGameHandler) handleFinishGameEvent(_ context.Context, event FinishGameEvent) error {
	h.lock.Lock()
	h.outputEvents = append(h.outputEvents, event)
	h.lock.Unlock()

	if len(h.outputEvents) == h.wantEventCount {
		go h.cancel()
	}
	return nil
}

func TestGameFinishStreamer(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), logutil.Trace, t.Name()))
	defer cancel()

	validInputEvents := []FinishGameEvent{
		{
			GameID: uuid.NewString(),
			Board:  chess.MakeEmptyBoard(true),
			Moves: []chess.HistMove{
				{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}},
			},
			WhitePlayer:  PlayerState{ID: 1, Name: "white1", Present: true},
			BlackPlayer:  PlayerState{ID: 2, Name: "black1", Present: true},
			GameMode:     ModeCorrespondence1,
			ReplayCause:  Checkmate,
			ReplayResult: WhiteWin,
		},
		{
			GameID:       uuid.NewString(),
			Board:        chess.MakeEmptyBoard(true),
			WhitePlayer:  PlayerState{ID: 3, Name: "white2", Present: true},
			BlackPlayer:  PlayerState{ID: 4, Name: "black2", Present: true},
			GameMode:     ModeTimed1Plus0,
			ReplayCause:  Forfeit,
			ReplayResult: BlackWin,
		},
		{
			GameID:       uuid.NewString(),
			Board:        chess.MakeEmptyBoard(true),
			WhitePlayer:  PlayerState{ID: 5, Name: "white3", Present: true},
			BlackPlayer:  PlayerState{ID: 6, Name: "black3", Present: true},
			GameMode:     ModeTimed3Plus2,
			ReplayCause:  Stalemate,
			ReplayResult: Draw,
		},
	}
	invalidInputEvents := []map[string]any{
		{
			"data": "invalid",
		},
		{
			"unknown": "field",
		},
	}

	handler := FakeGameHandler{
		cancel:         cancel,
		wantEventCount: len(validInputEvents),
	}
	stream := GameFinishStreamer{
		Context:               ctx,
		Services:              &services,
		Concurrency:           2,
		HandleFinishGameEvent: handler.handleFinishGameEvent,
	}

	// when
	for _, event := range invalidInputEvents {
		rdb := &services.Redis
		err := rdb.Queue.XAdd(ctx, &redis.XAddArgs{
			Stream: rdb.FinishGameStreamKey,
			Values: event,
		}).Err()
		require.NoError(t, err)
	}
	for _, event := range validInputEvents {
		services.PushFinishGameEvent(ctx, event)
	}
	stream.ReadGameFinishEvents()

	// then
	assert.ElementsMatch(t, validInputEvents, handler.outputEvents)
}

func TestInsertGameResult(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	for _, test := range []struct {
		name       string
		result     GameResult
		wantElos   []db.SelectUserModeElosByIdsRow
		wantReplay db.Replay
		wantChange GRChangeSet
		wantError  error
	}{
		{
			name:   "draw by stalemate",
			result: GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Stalemate, ReplayResult: Draw, ReplayMode: ModeTimed1Plus0},
			wantElos: []db.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1050, HighestElo: 1050, Draws: 1, Wins: 6, Losses: 5}, // update while maintaing old highest elo
				{UserID: testUser1.ID, Elo: 1000, HighestElo: 1000, Draws: 1},                     // insert
			},
			wantReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "DRAW",
				Cause:       "STALEMATE",
				Mode:        "TIMED_1+0",
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    1050,
				BlackElo:    1000,
			},
			wantChange: GRChangeSet{},
		},
		{
			name:   "white wins by checkmate",
			result: GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Checkmate, ReplayResult: WhiteWin, ReplayMode: ModeCorrespondence1},
			wantElos: []db.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},  // insert
				{UserID: testUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1}, // insert with elo lower than start elo
			},
			wantReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "WHITE_WINS",
				Cause:       "CHECKMATE",
				Mode:        "CORRESPONDENCE_1",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    1015,
				BlackElo:    985,
			},
			wantChange: GRChangeSet{WinID: testUser0.ID, LoseID: testUser1.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
		{
			name:   "black wins by forfeit",
			result: GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Forfeit, ReplayResult: BlackWin, ReplayMode: ModeCorrespondence7},
			wantElos: []db.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 985, HighestElo: 1000, Wins: 2, Losses: 3}, // update while setting new highest elo
				{UserID: testUser1.ID, Elo: 1015, HighestElo: 1015, Wins: 1},           // update
			},
			wantReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "BLACK_WINS",
				Cause:       "FORFEIT",
				Mode:        "CORRESPONDENCE_7",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    985,
				BlackElo:    1015,
			},
			wantChange: GRChangeSet{WinID: testUser1.ID, LoseID: testUser0.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
		{
			name:      "cannot insert already persisted game result",
			result:    GameResult{GameID: itest.ConstantGameID},
			wantError: GameResultNoopErr,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			services := SetupServicesTest(t, itest.RWPostgres)
			defer services.Close()

			// when
			cs, err := insertGameResult(ctx, services.Query(), time.Now(), test.result)

			if test.wantError != nil {
				require.Equal(t, test.wantError, err)
				return
			}
			require.NoError(t, err)

			// then
			rowElos, err := services.Query().SelectUserModeElosByIds(ctx, db.SelectUserModeElosByIdsParams{
				ID:   []int64{test.result.WhiteID, test.result.BlackID},
				Mode: db.ModeEnum(test.result.ReplayMode.String()),
			})
			require.NoError(t, err)

			assert.Equal(t, test.wantElos, rowElos)

			replay, err := services.Query().SelectReplayRowByID(ctx, cs.ReplayID)
			require.NoError(t, err)

			testutil.Equal(t, test.wantReplay, replay, cmpopts.IgnoreFields(db.Replay{}, "ID", "PlayedOn"))

			cs.ReplayID = 0
			cs.WinEloDiff = math.Round(cs.WinEloDiff)
			cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
			assert.Equal(t, test.wantChange, cs)
		})
	}
}
