package gameplay

import (
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/database/query"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/testutil"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameoverTest(t alog.TestLogger) (*GameOverService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewGameoverService(
		infra.Database,
		infra.Redis,
		producers.NewRiverProducer(&producers.NoopRiverClient{}),
		pubsub.NewSyncBroadcaster(infra.Redis),
		gamestate.NewChessRepoService(infra.Redis),
	)

	return services, infra
}

func TestInsertFinishedGameEvent(t *testing.T) {
	ctx := t.Context()

	testUser0 := itest.TestUser[0]
	testUser1 := itest.TestUser[1]
	newGameID := model.NewGameID()
	newGameIDGuest := model.NewGameID()

	tests := []struct {
		name            string
		event           model.FinishGameEvent
		wantLeaderboard []redis.Z
	}{
		{
			name: "InsertFinishedGame",
			event: model.FinishGameEvent{
				GameID:       newGameID,
				InitialBoard: chess.NewEmptyBoard(true),
				Moves:        []chess.HistMove{},
				WhitePlayer:  testUser0.ID, // winner
				BlackPlayer:  testUser1.ID, // loser
				ReplayMode:   model.ModeCorrespondence1,
				ReplayCause:  model.Checkmate,
				ReplayResult: model.WhiteWin,
			},
			wantLeaderboard: []redis.Z{
				{
					Member: strconv.Itoa(int(testUser0.ID)),
					Score:  15,
				},
				{
					Member: strconv.Itoa(int(testUser1.ID)),
					Score:  -15,
				},
			},
		},
		{
			name: "inserting already inserted finished game",
			event: model.FinishGameEvent{
				GameID:       model.GameID(itest.FirstReplayGameID),
				InitialBoard: chess.NewEmptyBoard(true),
				Moves:        []chess.HistMove{},
				// used only for validation
				WhitePlayer: testUser0.ID,
				BlackPlayer: testUser1.ID,
				// enum fields are ignored on a noop insertion.
				ReplayMode:   model.ModeCorrespondence1,
				ReplayCause:  model.Forfeit,
				ReplayResult: model.BlackWin,
			},
			wantLeaderboard: []redis.Z{}, // leaderboard is empty because it will not be updated since stats do not change
		},
		{
			name: "inserting a game with a guest",
			event: model.FinishGameEvent{
				GameID:       newGameIDGuest,
				InitialBoard: chess.NewEmptyBoard(true),
				Moves:        []chess.HistMove{},
				WhitePlayer:  testUser0.ID, // non-guest winner
				BlackPlayer:  -10,          // guest loser
				ReplayMode:   model.ModeCorrespondence1,
				ReplayCause:  model.Forfeit,
				ReplayResult: model.BlackWin,
			},
			wantLeaderboard: []redis.Z{}, // leaderboard is empty because it will not be updated since stats do not change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupGameoverTest(t)
			defer testinfra.Close()

			_, err := services.HandleFinishedGame(ctx, tt.event)
			require.NoError(t, err)

			modeLbZSet := cache.FmtLeaderboardZSet(tt.event.ReplayMode.String())
			leaderboardResp, err := testinfra.Redis.PrimaryClient.ZRevRangeWithScores(ctx, modeLbZSet, 0, 2).Result()
			require.NoError(t, err)

			assert.Equal(t, tt.wantLeaderboard, leaderboardResp)
		})
	}
}

func TestInsertGameResult(t *testing.T) {
	ctx := t.Context()

	testUser0 := itest.TestUser[0]
	testUser1 := itest.TestUser[1]

	now := time.Now()

	tests := []struct {
		name         string
		resultInput  GameResult
		wantUserElos []query.SelectUserModeElosByIDsRow
		wantReplay   query.Replay
		wantChange   GameResultChangeSet
	}{
		{
			name: "draw by stalemate",
			resultInput: GameResult{
				GameID:       "game1",
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  model.Stalemate,
				ReplayResult: model.Draw,
				ReplayMode:   model.ModeTimed1Plus0,
				InsertedTime: now,
			},
			wantUserElos: []query.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1050, HighestElo: 1050, Draws: 1, Wins: 6, Losses: 5}, // update while maintaing old highest elo
				{UserID: testUser1.ID, Elo: 1000, HighestElo: 1000, Draws: 1},                     // insert
			},
			wantReplay: query.Replay{
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
				ReplayCause:  model.Checkmate,
				ReplayResult: model.WhiteWin,
				ReplayMode:   model.ModeCorrespondence1,
				InsertedTime: now,
			},
			wantUserElos: []query.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1015, HighestElo: 1015, Wins: 1},  // insert
				{UserID: testUser1.ID, Elo: 985, HighestElo: 1000, Losses: 1}, // insert with elo lower than start elo
			},
			wantReplay: query.Replay{
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
				ReplayCause:  model.Forfeit,
				ReplayResult: model.BlackWin,
				ReplayMode:   model.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []query.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 985, HighestElo: 1000, Wins: 2, Losses: 3}, // update while setting new highest elo
				{UserID: testUser1.ID, Elo: 1015, HighestElo: 1015, Wins: 1},           // update
			},
			wantReplay: query.Replay{
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
				GameID:       model.GameID(itest.FirstReplayGameID),
				WhiteID:      testUser0.ID,
				BlackID:      testUser1.ID,
				ReplayCause:  model.Forfeit,
				ReplayResult: model.BlackWin,
				ReplayMode:   model.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []query.SelectUserModeElosByIDsRow{
				{UserID: testUser0.ID, Elo: 1000, HighestElo: 1000, Wins: 2, Losses: 2, Draws: 0}, // no update
			},
			wantReplay: query.Replay{
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
				ReplayCause:  model.Forfeit,
				ReplayResult: model.BlackWin,
				ReplayMode:   model.ModeCorrespondence7,
				InsertedTime: now,
			},
			wantUserElos: []query.SelectUserModeElosByIDsRow(nil), // not inserted.
			wantReplay: query.Replay{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupGameoverTest(t)
			defer testinfra.Close()

			changeSet, err := services.InsertGameResult(ctx, tt.resultInput)
			require.NoError(t, err)

			userElos, err := testinfra.Querier().SelectUserModeElosByIDs(ctx, query.SelectUserModeElosByIDsParams{
				ID:   []int64{tt.resultInput.WhiteID, tt.resultInput.BlackID},
				Mode: query.ModeEnum(tt.resultInput.ReplayMode.String()),
			})
			require.NoError(t, err)

			assert.Equal(t, tt.wantUserElos, userElos)

			userReplay, err := testinfra.Querier().SelectReplayRowByID(ctx, changeSet.ReplayID)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantReplay, userReplay, cmpopts.IgnoreFields(query.Replay{}, "ID", "PlayedOn", "PlayedOnAsDays", "TurnCount", "Rating"))

			changeSet.WinEloDiff = math.Round(changeSet.WinEloDiff)
			changeSet.LoseEloDiff = math.Round(changeSet.LoseEloDiff)
			testutil.Equal(t, tt.wantChange, changeSet, cmpopts.IgnoreFields(GameResultChangeSet{}, "ReplayID"))
		})
	}
}
