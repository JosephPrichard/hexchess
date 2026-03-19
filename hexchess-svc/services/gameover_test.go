package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type fakeGameEventHandler struct {
	lock           sync.Mutex
	cancel         func()
	outputEvents   []FinishGameEvent
	wantEventCount int
}

func (h *fakeGameEventHandler) handleFinishGameEvent(_ context.Context, event FinishGameEvent) error {
	h.lock.Lock()
	h.outputEvents = append(h.outputEvents, event)
	h.lock.Unlock()

	if len(h.outputEvents) == h.wantEventCount {
		go h.cancel()
	}
	return nil
}

func TestGameFinishStreamer_FakeGameEventHandler(t *testing.T) {
	t.Parallel()

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
			ReplayMode:   ModeCorrespondence1,
			ReplayCause:  Checkmate,
			ReplayResult: WhiteWin,
		},
		{
			GameID:       uuid.NewString(),
			Board:        chess.MakeEmptyBoard(true),
			WhitePlayer:  PlayerState{ID: 3, Name: "white2", Present: true},
			BlackPlayer:  PlayerState{ID: 4, Name: "black2", Present: true},
			ReplayMode:   ModeTimed1Plus0,
			ReplayCause:  Forfeit,
			ReplayResult: BlackWin,
		},
		{
			GameID:       uuid.NewString(),
			Board:        chess.MakeEmptyBoard(true),
			WhitePlayer:  PlayerState{ID: 5, Name: "white3", Present: true},
			BlackPlayer:  PlayerState{ID: 6, Name: "black3", Present: true},
			ReplayMode:   ModeTimed3Plus2,
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

	eventHandler := fakeGameEventHandler{
		cancel:         cancel,
		wantEventCount: len(validInputEvents),
	}

	stream := MakeFinishGameStreamer(ctx, &services)
	stream.HandleMessage = eventHandler.handleFinishGameEvent
	stream.StreamKey = services.Redis.FinishGameStreamKey

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
	stream.EventLoop()

	assert.ElementsMatch(t, validInputEvents, eventHandler.outputEvents)
}

func TestInsertFinishedGame(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]
	newGameID := uuid.NewString()

	for _, test := range []struct {
		name            string
		event           FinishGameEvent
		wantLeaderboard []string
		wantGameOutput  *pb.GameOutput
	}{
		{
			name: "insert finished game",
			event: FinishGameEvent{
				GameID:       newGameID,
				Board:        chess.MakeEmptyBoard(true),
				Moves:        []chess.HistMove{},
				WhitePlayer:  PlayerState{ID: testUser0.ID, Present: true}, // winner
				BlackPlayer:  PlayerState{ID: testUser1.ID, Present: true}, //loser
				ReplayMode:   ModeCorrespondence1,
				ReplayCause:  Checkmate,
				ReplayResult: WhiteWin,
			},

			wantLeaderboard: []string{
				strconv.Itoa(int(testUser0.ID)),
				strconv.Itoa(int(testUser1.ID)),
			},
			wantGameOutput: &pb.GameOutput{
				GameId: newGameID,
				Value: &pb.GameOutput_Replay{Replay: &pb.ReplayEntity{
					WhiteId:      testUser0.ID,
					BlackId:      testUser1.ID,
					WhiteName:    "user1",
					BlackName:    "user2",
					WhiteCountry: "us",
					BlackCountry: "us",
					Mode:         ModeCorrespondence1.String(),
					Cause:        Checkmate.String(),
					Result:       WhiteWin.String(),
					WinEloDiff:   15,
					LoseEloDiff:  -15,
					WhiteElo:     1015,
					BlackElo:     985,
					WhiteEloDiff: 15,
					BlackEloDiff: -15,
				}},
			},
		},
		{
			name: "inserting already inserted finished game",
			event: FinishGameEvent{
				GameID: itest.FirstReplayGameID,
				Board:  chess.MakeEmptyBoard(true),
				Moves:  []chess.HistMove{},
				// used only for validat
				WhitePlayer: PlayerState{ID: testUser0.ID, Present: true}, // winner
				BlackPlayer: PlayerState{ID: testUser1.ID, Present: true}, //loser
				// enum fields are ignored on a noop insertion.
				ReplayMode:   ModeCorrespondence1,
				ReplayCause:  Forfeit,
				ReplayResult: BlackWin,
			},
			wantLeaderboard: []string{}, // leaderboard is empty because it will not be updated if stats do not change.,
			wantGameOutput: &pb.GameOutput{
				GameId: itest.FirstReplayGameID,
				Value: &pb.GameOutput_Replay{Replay: &pb.ReplayEntity{
					WhiteId:      testUser0.ID,
					BlackId:      testUser1.ID,
					WhiteName:    "user1",
					BlackName:    "user2",
					WhiteCountry: "us",
					BlackCountry: "us",
					Mode:         ModeCorrespondence7.String(),
					Cause:        Checkmate.String(),
					Result:       WhiteWin.String(),
					WinEloDiff:   30,
					LoseEloDiff:  -30,
					WhiteElo:     1000,
					BlackElo:     1000,
					WhiteEloDiff: 30,
					BlackEloDiff: -30,
				}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {

			services := SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			stream := MakeFinishGameStreamer(ctx, &services)
			stream.StreamKey = services.Redis.FinishGameStreamKey

			lb := LocalBroadcasters{GamesCaster: MakeMultiCasterMap("testing-map", time.Hour*1)}
			<-lb.ListenGameMessages(services.Redis)

			subChan := make(chan []byte, 1)
			lb.GamesCaster.Subscribe(test.event.GameID, subChan) // expect the game event to come on the following gameID (derived from input) channel. test times out and fails if it does not.

			require.NoError(t, services.insertFinishedGameEvent(ctx, test.event))

			modeLbZSet := services.getLeaderboardZSet(test.event.ReplayMode.String())
			leaderboard, err := services.Redis.Cache.ZRevRange(ctx, modeLbZSet, 0, 2).Result()
			require.NoError(t, err)

			assert.Equal(t, test.wantLeaderboard, leaderboard)

			output := &pb.GameOutput{}
			require.NoError(t, proto.Unmarshal(<-subChan, output))
			testutil.Equal(t, test.wantGameOutput, output, protocmp.Transform(), protocmp.IgnoreFields(&pb.ReplayEntity{}, "id", "played_on"))
		})
	}
}

func TestInsertGameResult(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	now := time.Now()

	for _, test := range []struct {
		name         string
		resultInput  GameResult
		wantUserElos []sqlc.SelectUserModeElosByIdsRow
		wantReplay   sqlc.Replay
		wantChange   GameResultChangeSet
	}{
		{
			name: "draw by stalemate",
			resultInput: GameResult{
				GameID:       "game1",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  Stalemate,
				ReplayResult: Draw,
				ReplayMode:   ModeTimed1Plus0,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1050, HighestElo: 1050, Draws: 1, Wins: 6, Losses: 5}, // update while maintaing old highest elo
				{UserID: testUser1.ID, Elo: 1000, HighestElo: 1000, Draws: 1},                     // insert
			},
			wantReplay: sqlc.Replay{
				GameID:      "game1",
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "DRAW",
				Cause:       "STALEMATE",
				Mode:        "TIMED_1+0",
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    1050,
				BlackElo:    1000,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{},
		},
		{
			name: "white wins by checkmate",
			resultInput: GameResult{
				GameID:       "game2",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  Checkmate,
				ReplayResult: WhiteWin,
				ReplayMode:   ModeCorrespondence1,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},  // insert
				{UserID: testUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1}, // insert with elo lower than start elo
			},
			wantReplay: sqlc.Replay{
				GameID:      "game2",
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "WHITE_WINS",
				Cause:       "CHECKMATE",
				Mode:        "CORRESPONDENCE_1",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    1015,
				BlackElo:    985,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{WinID: testUser0.ID, LoseID: testUser1.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
		{
			name: "black wins by forfeit",
			resultInput: GameResult{
				GameID:       "game3",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  Forfeit,
				ReplayResult: BlackWin,
				ReplayMode:   ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 985, HighestElo: 1000, Wins: 2, Losses: 3}, // update while setting new highest elo
				{UserID: testUser1.ID, Elo: 1015, HighestElo: 1015, Wins: 1},           // update
			},
			wantReplay: sqlc.Replay{
				GameID:      "game3",
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "BLACK_WINS",
				Cause:       "FORFEIT",
				Mode:        "CORRESPONDENCE_7",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    985,
				BlackElo:    1015,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{WinID: testUser1.ID, LoseID: testUser0.ID, WinEloDiff: 15, LoseEloDiff: -15},
		},
		{
			name: "inserting already persisted game result",
			resultInput: GameResult{
				GameID:       itest.FirstReplayGameID,
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  Forfeit,
				ReplayResult: BlackWin,
				ReplayMode:   ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1000, HighestElo: 1000, Wins: 2, Losses: 2, Draws: 0}, // no update
			},
			wantReplay: sqlc.Replay{
				GameID:      itest.FirstReplayGameID,
				WhiteID:     1,
				BlackID:     2,
				Mode:        "CORRESPONDENCE_7",
				Result:      "WHITE_WINS",
				Cause:       "CHECKMATE",
				WinEloDiff:  30,
				LoseEloDiff: -30,
				WhiteElo:    1000,
				BlackElo:    1000,
				PlayedOn:    pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
			},
			wantChange: GameResultChangeSet{ReplayID: 1, AlreadyExists: true},
		},
	} {
		t.Run(test.name, func(t *testing.T) {

			services := SetupServicesTest(t, itest.RWPostgres)
			defer services.Close()

			changeSet, err := insertGameResult(ctx, services.DB.Queries(), test.resultInput)
			require.NoError(t, err)

			userElos, err := services.DB.Queries().SelectUserModeElosByIds(ctx, sqlc.SelectUserModeElosByIdsParams{
				ID:   []int64{test.resultInput.WhiteID, test.resultInput.BlackID},
				Mode: sqlc.ModeEnum(test.resultInput.ReplayMode.String()),
			})
			require.NoError(t, err)

			assert.Equal(t, test.wantUserElos, userElos)

			replay, err := services.DB.Queries().SelectReplayRowByID(ctx, changeSet.ReplayID)
			require.NoError(t, err)

			testutil.Equal(t, test.wantReplay, replay, cmpopts.IgnoreFields(sqlc.Replay{}, "ID"))

			changeSet.WinEloDiff = math.Round(changeSet.WinEloDiff)
			changeSet.LoseEloDiff = math.Round(changeSet.LoseEloDiff)
			testutil.Equal(t, test.wantChange, changeSet, cmpopts.IgnoreFields(GameResultChangeSet{}, "ReplayID"))
		})
	}
}
