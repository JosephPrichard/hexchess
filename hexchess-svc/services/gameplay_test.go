package svc

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinGame_JoinWhite(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := uuid.NewString()
	inState := MakeChessState(StateSetup{ID: gameID, Mode: ModeCorrespondence1, FirstColor: Random})
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
	AssertRedisChessState(t, &services, *updatedState, ChessMetaCmpOpt)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := uuid.NewString()
	inState := MakeChessState(StateSetup{
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
	AssertRedisChessState(t, &services, inState, ChessMetaCmpOpt)
}

func TestAttemptUndo(t *testing.T) {
	t.Parallel()

	// given
	testID1 := "test1_" + uuid.NewString()
	testID2 := "test2_" + uuid.NewString()
	inState1 := MakeChessState(StateSetup{
		ID:         testID1,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Name: "white", Present: true},
		Black:      PlayerState{ID: 2, Name: "black", Present: true},
	})
	inState2 := MakeChessState(StateSetup{
		ID:         testID2,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Name: "white", Present: true},
		Black:      PlayerState{ID: 2, Name: "black", Present: true},
	})
	inState2.Game.Moves = []chess.HistMove{{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}}}

	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()
	
	for _, state := range []ChessState{
		inState1, 
		inState2,
	} {
		if err := services.SetChessState(context.Background(), state.ID, &state); err != nil {
			t.Fatalf("failed initialize testing s: %v", err)
		}
	}

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
			name:   "trying to undo an invalid game",
			gameID: uuid.NewString(),
			tests: []subTest{
				{
					wantErr: ErrNoChessState,
				},
			},
		},
		{
			name:   "creating then accepting an undo",
			gameID: testID2,
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Name: "black", Present: true},
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoAccept,
					player:    PlayerState{ID: 1, Name: "white", Present: true},
					wantState: makeWantState(&inState2, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "creating then rejecting own undo",
			gameID: testID1,
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Name: "black", Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoReject,
					player:    PlayerState{ID: 2, Name: "black", Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "creating then rejecting undo",
			gameID: testID1,
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Name: "black", Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:      UndoReject,
					player:    PlayerState{ID: 1, Name: "white", Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{} }),
				},
			},
		},
		{
			name:   "rejecting and accepting an undo without creating",
			gameID: testID1,
			tests: []subTest{
				{
					kind:    UndoReject,
					player:  PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
			},
		},
		{
			name:   "creating an undo, accepting it as the creator, then accepting it with no previous moves",
			gameID: testID1,
			tests: []subTest{
				{
					kind:      UndoCreate,
					player:    PlayerState{ID: 2, Name: "black", Present: true},
					wantState: makeWantState(&inState1, func(s *ChessState) { s.UndoState = UndoState{UndoID: 2} }),
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 2, Name: "black", Present: true},
					wantErr: ErrUndoNoop,
				},
				{
					kind:    UndoAccept,
					player:  PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoMoveUndo,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			for _, subTest := range test.tests {
				// when
				state, err := services.AttemptGameUndo(ctx, test.gameID, subTest.player, subTest.kind)

				// then
				assert.Equal(t, subTest.wantErr, err)
				
				if subTest.wantErr == nil {
					cmptOpts := cmpopts.IgnoreFields(ChessState{}, "Game", "Touch")
					AssertChessState(t, subTest.wantState, state, cmptOpts)
					AssertRedisChessState(t, &services, *state, cmptOpts)
				}
			}
		})
	}
}

func TestMakeMove(t *testing.T) {
	t.Parallel()

	// given
	stateWhiteTurn := MakeChessState(StateSetup{
		ID:         "test1",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
	})
	stateEnded := MakeChessState(StateSetup{
		ID:         "test2",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	stateNotStarted := MakeChessState(StateSetup{
		ID:         "test3",
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
	})
	stateBlackIntoCheckmate := MakeChessState(StateSetup{
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

	stateWtMutated := stateWhiteTurn.DeepCopy()
	stateWtMutated.Game.MakeMove(chess.MoveStr("b1", "b2"))
	stateWtMutated.Game.InitPieceMoves()
	stateWtMutated.Game.ClearTables()
	stateWtMutated.Game.Moves = []chess.HistMove{
		{PieceMove: chess.PieceMove{Piece: chess.WhitePawn, From: chess.HexStr("b1"), To: chess.HexStr("b2")}, Notation: "Pb2"},
	}

	stateCheckmateMutated := stateBlackIntoCheckmate.DeepCopy()
	stateCheckmateMutated.Game.MakeMove(chess.MoveStr("a2", "a1"))
	stateCheckmateMutated.Game.InitPieceMoves()
	stateCheckmateMutated.Game.ClearTables()
	stateCheckmateMutated.Game.Moves = []chess.HistMove{
		{PieceMove: chess.PieceMove{Piece: chess.BlackQueen, From: chess.HexStr("a2"), To: chess.HexStr("a1")}, Notation: "qa1"},
	}
	stateCheckmateMutated.EndState = Finished

	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	for _, state := range []ChessState{
		stateWhiteTurn,
		stateEnded,
		stateNotStarted,
		stateBlackIntoCheckmate,
	} {
		if err := services.SetChessState(context.Background(), state.ID, &state); err != nil {
			t.Fatalf("failed initialize test state: %v", err)
		}
	}

	for _, test := range []struct {
		name        string
		move        chess.Move
		stateID     string
		player      PlayerState
		wantErr     error
		wantEndKind EndKind
		wantState   *ChessState
	}{
		{
			name:   "game doesn't exist",
			stateID: uuid.NewString(),
			wantErr: ErrNoChessState,
		},
		{
			name:      "invalid turn",
			move:      chess.Move{To: chess.Hex{File: 1}},
			stateID:   stateWhiteTurn.ID,
			player:    stateWhiteTurn.BlackPlayer,
			wantErr:   ErrTurn{GameID: "test1", PlayerID: 2, CurrID: 1},
			wantState: &stateWhiteTurn,
		},
		{
			name:      "invalid ended move",
			move:      chess.Move{To: chess.Hex{File: 1}},
			stateID:   stateEnded.ID,
			player:    stateEnded.WhitePlayer,
			wantErr:   ErrFinishedGame{GameID: "test2"},
			wantState: &stateEnded,
		},
		{
			name:      "invalid not started move",
			move:      chess.Move{To: chess.Hex{File: 1}},
			stateID:   stateNotStarted.ID,
			player:    stateNotStarted.WhitePlayer,
			wantErr:   ErrStartedGame{GameID: "test3"},
			wantState: &stateNotStarted,
		},
		{
			name:    "invalid move",
			move:    chess.Move{To: chess.Hex{File: 1}},
			stateID: stateWhiteTurn.ID,
			player:  stateWhiteTurn.WhitePlayer,
			wantErr: ErrInvalidMove{
				GameID:    "test1",
				PlayerID:  1,
				Violation: chess.MoveError{Kind: chess.MoveErrIllegalTarget, Move: chess.Move{To: chess.Hex{File: 1}}},
			},
			wantState: &stateWhiteTurn,
		},
		{
			name:      "valid move as white",
			move:      chess.MoveStr("b1", "b2"), // valid move
			stateID:   stateWhiteTurn.ID,
			player:    stateWhiteTurn.WhitePlayer,
			wantState: &stateWtMutated,
		},
		{
			name:        "valid move as black into checkmate",
			move:        chess.MoveStr("a2", "a1"), // valid move
			stateID:     stateBlackIntoCheckmate.ID,
			player:      stateBlackIntoCheckmate.BlackPlayer,
			wantEndKind: Finished,
			wantState:   &stateCheckmateMutated,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// when
			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)
			moveResult, err := services.MakeGameMove(ctx, test.stateID, test.player, test.move)

			// then
			assert.Equal(t, test.wantErr, err)

			if test.wantState != nil {
				AssertRedisChessState(t, &services, *test.wantState, ChessMetaCmpOpt)
			}
			if test.wantState != nil && moveResult.State != nil {
				AssertChessState(t, *test.wantState, moveResult.State, ChessMetaCmpOpt)
			}
		})
	}
}

func TestForfeit_Errors(t *testing.T) {
	t.Parallel()

	// given
	gameID1 := "testing-id1-" + uuid.NewString()
	gameID2 := "testing-id2-" + uuid.NewString()
	gameID3 := "testing-id2-" + uuid.NewString()

	state1 := MakeChessState(StateSetup{
		ID:         gameID1,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
		Black:      PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	state2 := MakeChessState(StateSetup{
		ID:         gameID2,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 2, Present: true},
	})
	state3 := MakeChessState(StateSetup{
		ID:         gameID3,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 3, Present: true},
		Black:      PlayerState{ID: 4, Present: true},
	})

	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	for _, state := range []ChessState{
		state1,
		state2,
		state3,
	} {
		if err := services.SetChessState(context.Background(), state.ID, &state); err != nil {
			t.Fatalf("failed initialize test state: %v", err)
		}
	}

	for _, test := range []struct {
		name    string
		gameID  string
		player  PlayerState
		wantErr error
	}{
		{
			name: "game doesn't exist",
			gameID: uuid.NewString(),
			wantErr: ErrNoChessState,
		},
		{
			name: "is already ended",
			gameID: gameID1,
			wantErr: ErrFinishedGame{GameID: gameID1},
		},
		{
			name: "cannot abort without being a player",
			gameID: gameID2,
			wantErr: ErrForfeitPlayer,
		},
		{
			name: "isn't a player",
			gameID: gameID3,
			wantErr: ErrForfeitPlayer,
		},
	} {
		t.Run(test.name, func(t *testing.T) { 
			// given
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			// when
			_, forfeitErr := services.EndGame(ctx, test.gameID, PlayerState{ID: 1, Present: true})

			// then
			assert.Equal(t, test.wantErr, forfeitErr)
		})
	}
}

func TestForfeit_Abort(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := uuid.NewString()
	inState := MakeChessState(StateSetup{
		ID:         gameID,
		Mode:       ModeCorrespondence1,
		FirstColor: Random,
		White:      PlayerState{ID: 1, Present: true},
	})

	// when
	require.NoError(t, services.SetChessState(ctx, gameID, &inState))
	endState, err := services.EndGame(ctx, gameID, inState.WhitePlayer)
	require.NoError(t, err)

	assert.Equal(t, Aborted, endState)

	// then
	wantState := inState.DeepCopy()
	wantState.EndState = Aborted
	AssertRedisChessState(t, &services, wantState, ChessMetaCmpOpt)

	// checks that the aborted state is removed from the games and users zsets
	gameKey := services.MakeGameKey(gameID)
	zRankErr := services.Redis.Cache.ZRank(ctx, services.Redis.GamesZSet, gameKey).Err()
	assert.Equal(t, redis.Nil, zRankErr)
	zRankErr = services.Redis.Cache.ZRank(ctx, services.GetUserGameZSet(inState.WhitePlayer.ID), gameKey).Err()
	assert.Equal(t, redis.Nil, zRankErr)
}

func TestForfeit(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	gameID := uuid.NewString()
	inState := MakeChessState(StateSetup{
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
	endState, err := services.EndGame(ctx, gameID, inState.BlackPlayer)
	require.NoError(t, err)

	assert.Equal(t, Finished, endState)

	// then
	wantState := inState.DeepCopy()
	wantState.EndState = Finished
	AssertRedisChessState(t, &services, wantState, ChessMetaCmpOpt)
}
