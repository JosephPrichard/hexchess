package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/domain"
	"hexchess-svc/itest"
	"hexchess-svc/pb"

	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"
	"math"
	"strconv"

	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestInsertFinishedGameEvent(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := itest.TestUser[0]
	testUser1 := itest.TestUser[1]
	newGameID := uuid.NewString()
	newGameIDGuest := uuid.NewString()

	for _, test := range []struct {
		name                string
		event               FinishedGame
		wantLeaderboard     []string
		wantBroadcastOutput *pb.GameOutput
	}{
		{
			name: "InsertFinishedGame",
			event: FinishedGame{
				GameID:       newGameID,
				Board:        chess.MakeEmptyBoard(true),
				Moves:        []chess.HistMove{},
				WhitePlayer:  domain.PlayerState{ID: testUser0.ID, Present: true}, // winner
				BlackPlayer:  domain.PlayerState{ID: testUser1.ID, Present: true}, // loser
				ReplayMode:   domain.ModeCorrespondence1,
				ReplayCause:  domain.Checkmate,
				ReplayResult: domain.WhiteWin,
			},
			wantLeaderboard: []string{
				strconv.Itoa(int(testUser0.ID)),
				strconv.Itoa(int(testUser1.ID)),
			},
			wantBroadcastOutput: &pb.GameOutput{
				GameId: newGameID,
				Value: &pb.GameOutput_Replay{Replay: &pb.Replay{
					WhiteId:      testUser0.ID,
					BlackId:      testUser1.ID,
					WhiteName:    "user1",
					BlackName:    "user2",
					WhiteCountry: "us",
					BlackCountry: "us",
					Mode:         domain.ModeCorrespondence1.String(),
					Cause:        domain.Checkmate.String(),
					Result:       domain.WhiteWin.String(),
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
			event: FinishedGame{
				GameID: itest.FirstReplayGameID,
				Board:  chess.MakeEmptyBoard(true),
				Moves:  []chess.HistMove{},
				// used only for validation
				WhitePlayer: domain.PlayerState{ID: testUser0.ID, Present: true},
				BlackPlayer: domain.PlayerState{ID: testUser1.ID, Present: true},
				// enum fields are ignored on a noop insertion.
				ReplayMode:   domain.ModeCorrespondence1,
				ReplayCause:  domain.Forfeit,
				ReplayResult: domain.BlackWin,
			},
			wantLeaderboard: []string{}, // leaderboard is empty because it will not be updated since stats do not change
			wantBroadcastOutput: &pb.GameOutput{
				GameId: itest.FirstReplayGameID,
				Value: &pb.GameOutput_Replay{Replay: &pb.Replay{
					WhiteId:      testUser0.ID,
					BlackId:      testUser1.ID,
					WhiteName:    "user1",
					BlackName:    "user2",
					WhiteCountry: "us",
					BlackCountry: "us",
					Mode:         domain.ModeCorrespondence7.String(),
					Cause:        domain.Checkmate.String(),
					Result:       domain.WhiteWin.String(),
					WinEloDiff:   30,
					LoseEloDiff:  -30,
					WhiteElo:     1000,
					BlackElo:     1000,
					WhiteEloDiff: 30,
					BlackEloDiff: -30,
				}},
			},
		},
		{
			name: "inserting a game with a guest",
			event: FinishedGame{
				GameID:       newGameIDGuest,
				Board:        chess.MakeEmptyBoard(true),
				Moves:        []chess.HistMove{},
				WhitePlayer:  domain.PlayerState{ID: testUser0.ID, Present: true}, // non-guest winner
				BlackPlayer:  domain.PlayerState{ID: -10, Present: true},          // guest loser
				ReplayMode:   domain.ModeCorrespondence1,
				ReplayCause:  domain.Forfeit,
				ReplayResult: domain.BlackWin,
			},
			wantLeaderboard: []string{}, // leaderboard is empty because it will not be updated since stats do not change
			wantBroadcastOutput: &pb.GameOutput{
				GameId: newGameIDGuest,
				Value: &pb.GameOutput_Replay{Replay: &pb.Replay{
					WhiteId:      testUser0.ID,
					BlackId:      0,
					WhiteName:    "user1",
					BlackName:    "",
					WhiteCountry: "us",
					BlackCountry: "",
					Mode:         domain.ModeCorrespondence1.String(),
					Cause:        domain.Forfeit.String(),
					Result:       domain.BlackWin.String(),
					WinEloDiff:   0,
					LoseEloDiff:  0,
					WhiteElo:     1000,
					BlackElo:     1000,
					WhiteEloDiff: 0,
					BlackEloDiff: 0,
				}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres, itest.Redis)
			defer services.Close()

			broadcasters := LocalBroadcasters{GamesCaster: MakeMultiCasterMap("testing-map", time.Hour*1)}
			<-broadcasters.ListenGameMessages(services.redis)

			// expect the game event to come on the following gameID (derived from input) channel. test times out and fails if it does not.
			subChan := make(chan []byte, 1)
			broadcasters.GamesCaster.Subscribe(test.event.GameID, subChan)

			err := services.InsertFinishedGame(ctx, test.event)
			require.NoError(t, err)

			modeLbZSet := services.leaderboardZSet(test.event.ReplayMode.String())
			leaderboard, err := services.redis.Cache.ZRevRange(ctx, modeLbZSet, 0, 2).Result()
			require.NoError(t, err)

			assert.Equal(t, test.wantLeaderboard, leaderboard)

			output := &pb.GameOutput{}
			require.NoError(t, proto.Unmarshal(<-subChan, output))
			testutil.Equal(t, test.wantBroadcastOutput, output, protocmp.Transform(), protocmp.IgnoreFields(&pb.Replay{}, "id", "played_on"))
		})
	}
}

func TestInsertGameResult(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := itest.TestUser[0]
	testUser1 := itest.TestUser[1]

	now := time.Now()

	for _, test := range []struct {
		name         string
		resultInput  GameResult
		wantUserElos []sqlc.SelectUserModeElosByIDsRow
		wantReplay   sqlc.Replay
		wantChange   GameResultChangeSet
	}{
		{
			name: "draw by stalemate",
			resultInput: GameResult{
				GameID:       "game1",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  domain.Stalemate,
				ReplayResult: domain.Draw,
				ReplayMode:   domain.ModeTimed1Plus0,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1050, HighestElo: 1050, Draws: 1, Wins: 6, Losses: 5}, // update while maintaing old highest elo
				{UserID: testUser1.ID, Elo: 1000, HighestElo: 1000, Draws: 1},                     // insert
			},
			wantReplay: sqlc.Replay{
				GameID:      "game1",
				WhiteID:     pgtype.Int8{Int64: testUser0.ID, Valid: true},
				BlackID:     pgtype.Int8{Int64: testUser1.ID, Valid: true},
				Result:      "DRAW",
				Cause:       "STALEMATE",
				Mode:        "TIMED_1+0",
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    1050,
				BlackElo:    1000,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{
				WhiteEloNext: 1050,
				BlackEloNext: 1000,
			},
		},
		{
			name: "white wins by checkmate",
			resultInput: GameResult{
				GameID:       "game2",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  domain.Checkmate,
				ReplayResult: domain.WhiteWin,
				ReplayMode:   domain.ModeCorrespondence1,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},  // insert
				{UserID: testUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1}, // insert with elo lower than start elo
			},
			wantReplay: sqlc.Replay{
				GameID:      "game2",
				WhiteID:     pgtype.Int8{Int64: testUser0.ID, Valid: true},
				BlackID:     pgtype.Int8{Int64: testUser1.ID, Valid: true},
				Result:      "WHITE_WINS",
				Cause:       "CHECKMATE",
				Mode:        "CORRESPONDENCE_1",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    1015,
				BlackElo:    985,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{
				WinID:        testUser0.ID,
				LoseID:       testUser1.ID,
				WinEloDiff:   15,
				LoseEloDiff:  -15,
				WhiteEloNext: 1015,
				BlackEloNext: 985,
			},
		},
		{
			name: "black wins by forfeit",
			resultInput: GameResult{
				GameID:       "game3",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  domain.Forfeit,
				ReplayResult: domain.BlackWin,
				ReplayMode:   domain.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 985, HighestElo: 1000, Wins: 2, Losses: 3}, // update while setting new highest elo
				{UserID: testUser1.ID, Elo: 1015, HighestElo: 1015, Wins: 1},           // update
			},
			wantReplay: sqlc.Replay{
				GameID:      "game3",
				WhiteID:     pgtype.Int8{Int64: testUser0.ID, Valid: true},
				BlackID:     pgtype.Int8{Int64: testUser1.ID, Valid: true},
				Result:      "BLACK_WINS",
				Cause:       "FORFEIT",
				Mode:        "CORRESPONDENCE_7",
				WinEloDiff:  15,
				LoseEloDiff: -15,
				WhiteElo:    985,
				BlackElo:    1015,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{
				WinID:        testUser1.ID,
				LoseID:       testUser0.ID,
				WinEloDiff:   15,
				LoseEloDiff:  -15,
				WhiteEloNext: 985,
				BlackEloNext: 1015,
			},
		},
		{
			name: "inserting already persisted game result",
			resultInput: GameResult{
				GameID:       itest.FirstReplayGameID,
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  domain.Forfeit,
				ReplayResult: domain.BlackWin,
				ReplayMode:   domain.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1000, HighestElo: 1000, Wins: 2, Losses: 2, Draws: 0}, // no update
			},
			wantReplay: sqlc.Replay{
				GameID:      itest.FirstReplayGameID,
				WhiteID:     pgtype.Int8{Int64: testUser0.ID, Valid: true},
				BlackID:     pgtype.Int8{Int64: testUser1.ID, Valid: true},
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
		{
			name: "game insert with guest players",
			resultInput: GameResult{
				GameID:       "game4",
				WhiteID:      -10,
				BlackID:      -20,
				ReplayCause:  domain.Forfeit,
				ReplayResult: domain.BlackWin,
				ReplayMode:   domain.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []sqlc.SelectUserModeElosByIDsRow(nil), // not inserted.
			wantReplay: sqlc.Replay{
				GameID:      "game4",
				WhiteID:     pgtype.Int8{},
				BlackID:     pgtype.Int8{},
				Result:      "BLACK_WINS",
				Cause:       "FORFEIT",
				Mode:        "CORRESPONDENCE_7",
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    0,
				BlackElo:    0,
				PlayedOn:    pgtype.Timestamptz{Time: now, Valid: true},
			},
			wantChange: GameResultChangeSet{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
			defer services.Close()

			changeSet, err := services.InsertGameResultTx(ctx, test.resultInput)
			require.NoError(t, err)

			userElos, err := services.db.Querier().SelectUserModeElosByIDs(ctx, sqlc.SelectUserModeElosByIDsParams{
				ID:   []int64{test.resultInput.WhiteID, test.resultInput.BlackID},
				Mode: sqlc.ModeEnum(test.resultInput.ReplayMode.String()),
			})
			require.NoError(t, err)

			assert.Equal(t, test.wantUserElos, userElos)

			replay, err := services.db.Querier().SelectReplayRowByID(ctx, changeSet.ReplayID)
			require.NoError(t, err)

			testutil.Equal(t, test.wantReplay, replay, cmpopts.IgnoreFields(sqlc.Replay{}, "ID", "PlayedOn"))

			changeSet.WinEloDiff = math.Round(changeSet.WinEloDiff)
			changeSet.LoseEloDiff = math.Round(changeSet.LoseEloDiff)
			testutil.Equal(t, test.wantChange, changeSet, cmpopts.IgnoreFields(GameResultChangeSet{}, "ReplayID"))
		})
	}
}
