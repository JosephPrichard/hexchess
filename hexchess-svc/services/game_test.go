package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/pkg/assertutil"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/ptr"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestJoinGame_JoinWhite(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-join-game")

	gameID := "test123"
	inState := MakeState(StateSetup{ID: gameID, Mode: ModeCorrespondence1, FirstColor: Random})
	inState.FirstColor = White
	player := MakePlayer(1, "name", "us")

	// when
	require.NoError(t, SetChessState(ctx, rdb, gameID, &inState))

	updatedState, err := JoinGame(ctx, rdb, gameID, player)
	require.NoError(t, err)

	// then
	wantState := inState.DeepCopy()
	wantState.WhitePlayer = player

	AssertChessState(t, wantState, updatedState, ChessMetaCmpOpt)
	AssertRedisChess(t, rdb, *updatedState, ChessMetaCmpOpt)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-join-game-both-players")

	gameID := "test123"
	inState := MakeState(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeNamePlayer(1, "white")),
		Black:      ptr.New(MakeNamePlayer(2, "name")),
	})

	// when
	require.NoError(t, SetChessState(ctx, rdb, gameID, &inState))

	resultState, err := JoinGame(ctx, rdb, gameID, MakeNamePlayer(3, "test"))
	require.NoError(t, err)

	// then
	AssertChessState(t, inState, resultState, ChessMetaCmpOpt)
	AssertRedisChess(t, rdb, inState, ChessMetaCmpOpt)
}

func TestAttemptUndo(t *testing.T) {
	inState1 := MakeState(StateSetup{
		ID:         "test1",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeNamePlayer(1, "white")),
		Black:      ptr.New(MakeNamePlayer(2, "black")),
	})
	inState2 := MakeState(StateSetup{
		ID:         "test2",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeNamePlayer(1, "white")),
		Black:      ptr.New(MakeNamePlayer(2, "black")),
	})
	inState2.Game.Moves = []chess.HistMove{{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}}}

	states := []ChessState{inState1, inState2}

	makeWantState := func(inState *ChessState, fn func(s *ChessState)) ChessState {
		state := inState.DeepCopy()
		fn(&state)
		return state
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
			name:   "creating then rejecting an undo",
			gameID: "test1",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    MakeIDPlayer(1),
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 1} }),
				},
				{
					kind:      UndoReject,
					player:    MakeIDPlayer(1),
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
					player:  MakeIDPlayer(1),
					wantErr: ErrNoUndo,
				},
				{
					kind:    UndoAccept,
					player:  MakeIDPlayer(1),
					wantErr: ErrNoUndo,
				},
			},
		},
		{
			name:   "creating then accepting an undo",
			gameID: "test2",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    MakeIDPlayer(1),
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{UndoID: 1} }),
				},
				{
					kind:      UndoAccept,
					player:    MakeIDPlayer(2),
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "creating an undo, accepting it as the creator, then accepting it with no previous moves",
			gameID: "test1",
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    MakeIDPlayer(2),
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:    UndoAccept,
					player:  MakeIDPlayer(2),
					wantErr: ErrUndoNoop,
				},
				{
					kind:    UndoAccept,
					player:  MakeIDPlayer(1),
					wantErr: ErrNoMoveUndo,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			rdb := db.BeforeRedisTest(t)
			defer rdb.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, "testing-undo")

			for _, state := range states {
				if err := SetChessState(ctx, rdb, state.ID, &state); err != nil {
					t.Fatalf("failed initialize test state: %v", err)
				}
			}

			for _, subTest := range test.tests {
				// when
				state, err := AttemptGameUndo(ctx, rdb, test.gameID, subTest.player, subTest.kind)

				// then
				assert.Equal(t, subTest.wantErr, err)
				if subTest.wantErr == nil {
					cmptOpts := cmpopts.IgnoreFields(ChessState{}, "Game", "Touch")
					AssertChessState(t, subTest.wantState, state, cmptOpts)
					AssertRedisChess(t, rdb, *state, cmptOpts)
				}
			}
		})
	}
}

func TestMakeMove(t *testing.T) {
	s1 := MakeState(StateSetup{
		ID:         "test1",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeIDPlayer(1)),
		Black:      ptr.New(MakeIDPlayer(2)),
	})
	s2 := MakeState(StateSetup{
		ID:         "test2",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeIDPlayer(3)),
		Black:      ptr.New(MakeIDPlayer(4)),
		Game: ptr.New(chess.MakeEmptyGame(false,
			chess.NotMove{Not: "f1", Piece: chess.WhiteKing},
			chess.NotMove{Not: "a2", Piece: chess.BlackQueen},
			chess.NotMove{Not: "h1", Piece: chess.BlackRook},
			chess.NotMove{Not: "f3", Piece: chess.BlackRook},
			chess.NotMove{Not: "f9", Piece: chess.BlackKing},
		)),
	})

	type subTest struct {
		pm      chess.Move
		state   ChessState
		player  PlayerState
		wantErr error
	}
	for _, test := range []struct {
		name  string
		tests []subTest
	}{
		{
			name: "invalid turn",
			tests: []subTest{
				{
					pm:      chess.Move{To: chess.Hex{File: 1}},
					state:   s1,
					player:  s1.BlackPlayer,
					wantErr: ErrTurn,
				},
			},
		},
		{
			name: "invalid move",
			tests: []subTest{
				{

					pm:      chess.Move{To: chess.Hex{File: 1}},
					state:   s1,
					player:  s1.WhitePlayer,
					wantErr: ErrInvalidMove,
				},
			},
		},
		{
			name: "invalid move",
			tests: []subTest{
				{
					pm:      chess.Move{To: chess.Hex{File: 1}},
					state:   s1,
					player:  s1.WhitePlayer,
					wantErr: ErrInvalidMove,
				},
			},
		},
		{
			name: "valid move as white THEN black",
			tests: []subTest{
				{
					pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.HexStr("b1"), To: chess.HexStr("b2")}, // valid move
					state:  s1,
					player: s1.WhitePlayer,
				},
				{
					pm:     chess.Move{Promotion: chess.QueenPromotion, From: chess.HexStr("a2"), To: chess.HexStr("a1")}, // valid move
					state:  s2,
					player: s2.BlackPlayer,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			ctx := context.WithValue(t.Context(), logutil.Trace, "testing-make-move")

			databases, closer := db.BeforeDbTest(t, true, InsertTestData)
			defer closer()

			for _, state := range []ChessState{s1, s2} {
				state.Game.InitPieceMoves()
				if err := SetChessState(ctx, databases.Rdb, state.ID, &state); err != nil {
					t.Fatalf("failed initialize test state: %v", err)
				}
			}

			for _, subTest := range test.tests {
				// when
				_, err := MakeGameMove(ctx, &databases, subTest.state.ID, subTest.player, subTest.pm)

				// then
				assert.Equal(t, subTest.wantErr, err)
			}
		})
	}
}

func TestForfeit_BlackForfeits(t *testing.T) {
	// given
	databases, closer := db.BeforeDbTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-forfeit")

	gameID := "test123"
	inState := MakeState(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      ptr.New(MakeIDPlayer(1)),
		Black:      ptr.New(MakeIDPlayer(2)),
	})
	inState.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	// when
	require.NoError(t, SetChessState(ctx, databases.Rdb, gameID, &inState))
	replayID, err := ForfeitGame(ctx, &databases, gameID, inState.BlackPlayer)
	require.NoError(t, err)

	// then
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
	wantState := inState.DeepCopy()
	wantState.IsEnded = true

	AssertRedisChess(t, databases.Rdb, wantState, ChessMetaCmpOpt)

	replay, err := databases.Pdb.Query.SelectReplayRowByID(ctx, replayID)
	require.NoError(t, err)
	assertutil.AssertEqualIgnoring(t, wantReplay, replay, cmpopts.IgnoreFields(db.Replay{}, "ID", "PlayedOn", "MoveHistory"))
}

func TestInsertGameResult(t *testing.T) {
	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-insert-game-result")

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
				{UserID: testUser0.ID, Elo: 1000},
				{UserID: testUser1.ID, Elo: 1000},
			},
			wantReplay: db.Replay{
				WhiteID:     testUser0.ID,
				BlackID:     testUser1.ID,
				Result:      "DRAW",
				Cause:       "STALEMATE",
				Mode:        "TIMED_1+0",
				WinEloDiff:  0,
				LoseEloDiff: 0,
				WhiteElo:    1000,
				BlackElo:    1000,
			},
			wantChange: GRChangeSet{},
		},
		{
			name:   "white wins by checkmate",
			result: GameResult{WhiteID: testUser0.ID, BlackID: testUser1.ID, ReplayCause: Checkmate, ReplayResult: WhiteWin, ReplayMode: ModeCorrespondence1},
			wantElos: []db.SelectUserModeElosByIdsRow{
				{UserID: testUser0.ID, Elo: 1015},
				{UserID: testUser1.ID, Elo: 985},
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
				{UserID: testUser0.ID, Elo: 985},
				{UserID: testUser1.ID, Elo: 1015},
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
			pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
			defer closer()

			// when
			cs, err := insertGameResult(ctx, pdb.Query, time.Now(), test.result)
			require.NoError(t, err)

			// then
			rowElos, err := pdb.Query.SelectUserModeElosByIds(ctx, db.SelectUserModeElosByIdsParams{
				ID:   []int64{test.result.WhiteID, test.result.BlackID},
				Mode: db.ModeEnum(test.result.ReplayMode.String()),
			})
			require.NoError(t, err)

			assert.Equal(t, test.wantElos, rowElos)

			replay, err := pdb.Query.SelectReplayRowByID(ctx, cs.ReplayID)
			require.NoError(t, err)

			assertutil.AssertEqualIgnoring(t, test.wantReplay, replay, cmpopts.IgnoreFields(db.Replay{}, "ID", "PlayedOn", "MoveHistory"))

			cs.ReplayID = 0
			cs.WinEloDiff = math.Round(cs.WinEloDiff)
			cs.LoseEloDiff = math.Round(cs.LoseEloDiff)
			assert.Equal(t, test.wantChange, cs)
		})
	}
}
