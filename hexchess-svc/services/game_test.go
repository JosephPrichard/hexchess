package svc

import (
	"context"
	"math"
	"testing"
	"time"

	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinGame_JoinWhite(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := "test123"
	inState := MakeChess(StateSetup{ID: gameID, Mode: ModeCorrespondence1, FirstColor: Random})
	inState.FirstColor = White
	player := MakePlayer(1, "name", "us")

	// when
	require.NoError(t, services.SetChessState(ctx, gameID, &inState))

	updatedState, err := services.JoinGame(ctx, gameID, player)
	require.NoError(t, err)

	// then
	wantState := inState.DeepCopy()
	wantState.WhitePlayer = player

	AssertChessState(t, wantState, updatedState, ChessMetaCmpOpt)
	AssertRedisChess(t, &services, *updatedState, ChessMetaCmpOpt)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := "test123"
	inState := MakeChess(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Name: "white", Present: true},
		Black:      PlayerState{ID: 2, Name: "black", Present: true},
	})

	// when
	require.NoError(t, services.SetChessState(ctx, gameID, &inState))

	resultState, err := services.JoinGame(ctx, gameID, PlayerState{ID: 3, Name: "testing", Present: true})
	require.NoError(t, err)

	// then
	AssertChessState(t, inState, resultState, ChessMetaCmpOpt)
	AssertRedisChess(t, &services, inState, ChessMetaCmpOpt)
}

func TestAttemptUndo(t *testing.T) {
	inState1 := MakeChess(StateSetup{
		ID:         "test1",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Name: "white", Present: true},
		Black:      PlayerState{ID: 2, Name: "black", Present: true},
	})
	inState2 := MakeChess(StateSetup{
		ID:         "test2",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Name: "white", Present: true},
		Black:      PlayerState{ID: 2, Name: "black", Present: true},
	})
	inState2.Game.Moves = []chess.HistMove{{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}}}

	ss := []ChessState{inState1, inState2}

	makeWantState := func(inState *ChessState, fn func(s *ChessState)) ChessState {
		s := inState.DeepCopy()
		fn(&s)
		return s
	}

	type subTest struct {
		kind      UndoKind
		player    PlayerState
		wantState ChessState
		wantErr   error
	}

	for _, test := range []struct {
		name   string
		gameID string
		tests  []subTest
	}{
		{
			name:   "creating then accepting an undo",
			gameID: "test2",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Present: true},
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoAccept,
					player:    PlayerState{ID: 1, Present: true},
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "creating then rejecting own undo",
			gameID: "test1",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoReject,
					player:    PlayerState{ID: 2, Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "creating then rejecting undo",
			gameID: "test1",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoReject,
					player:    PlayerState{ID: 1, Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "rejecting and accepting an undo without creating",
			gameID: "test1",
			tests: []subTest{
				{
					kind:    UndoReject,
					player:  PlayerState{ID: 1, Present: true},
					wantErr: ErrNoUndo,
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 1, Present: true},
					wantErr: ErrNoUndo,
				},
			},
		},
		{
			name:   "creating an undo, accepting it as the creator, then accepting it with no previous moves",
			gameID: "test1",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 2, Present: true},
					wantErr: ErrUndoNoop,
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 1, Present: true},
					wantErr: ErrNoMoveUndo,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			services := SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			for _, cs := range ss {
				if err := services.SetChessState(ctx, cs.ID, &cs); err != nil {
					t.Fatalf("failed initialize testing s: %v", err)
				}
			}

			for _, subTest := range test.tests {
				// when
				cs, err := services.AttemptGameUndo(ctx, test.gameID, subTest.player, subTest.kind)

				// then
				assert.Equal(t, subTest.wantErr, err)
				if subTest.wantErr == nil {
					cmptOpts := cmpopts.IgnoreFields(ChessState{}, "Game", "Touch")
					AssertChessState(t, subTest.wantState, cs, cmptOpts)
					AssertRedisChess(t, &services, *cs, cmptOpts)
				}
			}
		})
	}
}

func TestMakeMove(t *testing.T) {
	stateWhiteTurn := MakeChess(StateSetup{
		ID:         "test1",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
	})
	stateEnded := MakeChess(StateSetup{
		ID:         "test2",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
		FinishState: EndState{
			Kind:        Finished,
			WinEloDiff:  15,
			LoseEloDiff: -15,
			Cause:       Checkmate,
			Result:      WhiteWin,
		},
	})
	stateNotStarted := MakeChess(StateSetup{
		ID:         "test3",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
	})
	stateBlackIntoCheckmate := MakeChess(StateSetup{
		ID:         "test4",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 3, Present: true},
		Black:      PlayerState{ID: 4, Present: true},
		Game: New(chess.MakeEmptyGame(false,
			chess.Place{Not: "f1", Piece: chess.WhiteKing},
			chess.Place{Not: "a2", Piece: chess.BlackQueen},
			chess.Place{Not: "h1", Piece: chess.BlackRook},
			chess.Place{Not: "f3", Piece: chess.BlackRook},
			chess.Place{Not: "f9", Piece: chess.BlackKing},
		)),
	})
	ss := []ChessState{stateWhiteTurn, stateEnded, stateNotStarted, stateBlackIntoCheckmate}

	for _, test := range []struct {
		name            string
		move            chess.Move
		sID             string
		player          PlayerState
		wantErr         error
		wantHasReplay   bool
		wantFinishState EndState
	}{
		{
			name:    "invalid turn",
			move:    chess.Move{To: chess.Hex{File: 1}},
			sID:     stateWhiteTurn.ID,
			player:  stateWhiteTurn.BlackPlayer,
			wantErr: ErrTurn{GameID: "test1", PlayerID: 2, CurrID: 1},
		},
		{
			name:    "invalid ended move",
			move:    chess.Move{To: chess.Hex{File: 1}},
			sID:     stateEnded.ID,
			player:  stateEnded.WhitePlayer,
			wantErr: ErrFinishedGame{GameID: "test2", Kind: Finished},
		},
		{
			name:    "invalid not started move",
			move:    chess.Move{To: chess.Hex{File: 1}},
			sID:     stateNotStarted.ID,
			player:  stateNotStarted.WhitePlayer,
			wantErr: ErrStartedGame{GameID: "test3"},
		},
		{
			name:   "invalid move",
			move:   chess.Move{To: chess.Hex{File: 1}},
			sID:    stateWhiteTurn.ID,
			player: stateWhiteTurn.WhitePlayer,
			wantErr: ErrInvalidMove{
				GameID:    "test1",
				PlayerID:  1,
				Violation: chess.MoveError{Kind: chess.MoveErrIllegalTarget, Move: chess.Move{To: chess.Hex{File: 1}}},
			},
		},
		{
			name:   "valid move as white",
			move:   chess.MoveStr("b1", "b2"), // valid move
			sID:    stateWhiteTurn.ID,
			player: stateWhiteTurn.WhitePlayer,
		},
		{
			name:   "valid move as black",
			move:   chess.MoveStr("a2", "a1"), // valid move
			sID:    stateBlackIntoCheckmate.ID,
			player: stateBlackIntoCheckmate.BlackPlayer,
			wantFinishState: EndState{
				Kind:        Finished,
				WinEloDiff:  30,
				LoseEloDiff: -30,
				Cause:       Checkmate,
				Result:      BlackWin,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			services := SetupServicesTest(t, itest.RWPostgres, itest.Redis, itest.Aws)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			for _, c := range ss {
				if err := services.SetChessState(ctx, c.ID, &c); err != nil {
					t.Fatalf("failed initialize testing state: %v", err)
				}
			}

			// when
			moveResult, err := services.MakeGameMove(ctx, test.sID, test.player, test.move)

			// then
			assert.Equal(t, test.wantErr, err)
			assert.Equal(t, test.wantHasReplay, moveResult.ReplayID != 0)
			if moveResult.State != nil {
				testutil.Equal(t, test.wantFinishState, moveResult.State.EndState, cmpopts.IgnoreFields(EndState{}, "ReplayID"))
			}
		})
	}
}

func TestForfeit_Errors(t *testing.T) {
	gameID := "test123"

	for _, test := range []struct {
		name    string
		s       ChessState
		player  PlayerState
		wantErr error
	}{
		{
			name: "is already ended",
			s: MakeChess(StateSetup{
				ID:         gameID,
				Mode:       ModeCorrespondence1,
				FirstColor: Random,
				White:      PlayerState{ID: 1, Present: true},
				Black:      PlayerState{ID: 2, Present: true},
				FinishState: EndState{
					Kind:   Finished,
					Cause:  Checkmate,
					Result: WhiteWin,
				},
			}),
			wantErr: ErrFinishedGame{GameID: "test123", Kind: Finished},
		},
		{
			name: "cannot abort without being a player",
			s: MakeChess(StateSetup{
				ID:         gameID,
				Mode:       ModeCorrespondence1,
				FirstColor: Random,
				White:      PlayerState{ID: 2, Present: true},
			}),
			wantErr: ErrForfeitPlayer,
		},
		{
			name: "isn't a player",
			s: MakeChess(StateSetup{
				ID:         gameID,
				Mode:       ModeCorrespondence1,
				FirstColor: Random,
				White:      PlayerState{ID: 3, Present: true},
				Black:      PlayerState{ID: 4, Present: true},
			}),
			wantErr: ErrForfeitPlayer,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			services := SetupServicesTest(t, itest.RWPostgres, itest.Redis)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			// when
			require.NoError(t, services.SetChessState(ctx, gameID, &test.s))
			_, forfeitErr := services.ForfeitGame(ctx, gameID, PlayerState{ID: 1, Present: true})

			// then
			assert.Equal(t, test.wantErr, forfeitErr)
		})
	}
}

func TestForfeit_Abort(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := "test123"
	inState := MakeChess(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
	})

	// when
	require.NoError(t, services.SetChessState(ctx, gameID, &inState))
	endState, err := services.ForfeitGame(ctx, gameID, inState.WhitePlayer)
	require.NoError(t, err)

	// then
	wantEndState := EndState{Kind: Aborted}

	wantState := inState.DeepCopy()
	wantState.EndState = wantEndState
	AssertRedisChess(t, &services, wantState, ChessMetaCmpOpt)

	// checks that the aborted state is removed from the games and users zsets
	zRankErr := services.Redis.Cache.ZRank(ctx, services.Redis.GamesZSet, makeGameKey(gameID)).Err()
	assert.Equal(t, redis.Nil, zRankErr)
	zRankErr = services.Redis.Cache.ZRank(ctx, services.makeUserGameZSet(inState.WhitePlayer.ID), makeGameKey(gameID)).Err()
	assert.Equal(t, redis.Nil, zRankErr)

	assert.Equal(t, wantEndState, endState)
}

func TestForfeit(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.RWPostgres, itest.Redis, itest.Aws)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := "test123"
	inState := MakeChess(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
	})
	inState.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	// when
	require.NoError(t, services.SetChessState(ctx, gameID, &inState))
	endState, err := services.ForfeitGame(ctx, gameID, inState.BlackPlayer)
	require.NoError(t, err)

	// then
	wantEndState := EndState{
		Kind:        Finished,
		WinEloDiff:  15,
		LoseEloDiff: -15,
		Cause:       Forfeit,
		Result:      WhiteWin,
	}
	testutil.Equal(t, wantEndState, endState, cmpopts.IgnoreFields(EndState{}, "ReplayID"))

	wantState := inState.DeepCopy()
	wantState.EndState = wantEndState
	AssertRedisChess(t, &services, wantState, ChessMetaCmpOpt)

	wantReplay := db.Replay{
		WhiteID:     1,
		BlackID:     2,
		Mode:        "CORRESPONDENCE_1",
		Result:      "WHITE_WINS",
		Cause:       "FORFEIT",
		WinEloDiff:  15,
		LoseEloDiff: -15,
		WhiteElo:    1015,
		BlackElo:    985,
	}
	replay, err := services.Query().SelectReplayRowByID(ctx, endState.ReplayID)
	require.NoError(t, err)
	testutil.Equal(t, wantReplay, replay, cmpopts.IgnoreFields(db.Replay{}, "ID", "PlayedOn"))
}

func TestInsertGameResult(t *testing.T) {
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	testUser0 := TestUserEntities[0]
	testUser1 := TestUserEntities[1]

	for _, test := range []struct {
		name       string
		result     GameResult
		wantElos   []db.SelectUserModeElosByIdsRow
		wantReplay db.Replay
		wantChange GRChangeSet
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
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			services := SetupServicesTest(t, itest.RWPostgres)
			defer services.Close()

			// when
			cs, err := insertGameResult(ctx, services.Query(), time.Now(), test.result)
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
