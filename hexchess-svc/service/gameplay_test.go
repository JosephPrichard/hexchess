package svc

import (
	"context"
	"hexchess-svc/hexchess"
	"hexchess-svc/itest"
	"hexchess-svc/model"

	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mutateGame(state *ChessState, fn func(s *ChessState)) *ChessState {
	s := state.DeepCopy()
	fn(&s)
	return &s
}

func seedGames(t *testing.T, services *HexchessServices, games ...*ChessState) {
	t.Helper()
	for _, g := range games {
		require.NoError(t, services.SetChessState(t.Context(), g.ID, g))
	}
}

func TestJoinGame(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	whiteGame := MakeChessState(StateSetup{
		ID:         "test-join-white-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.White,
	})
	fullGame := MakeChessState(StateSetup{
		ID:         "test-join-full-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	missingGameID := "test-join-missing-" + uuid.NewString()

	seedGames(t, services, whiteGame, fullGame)

	tests := []struct {
		name       string
		gameID     string
		joinPlayer model.PlayerState
		wantGame   *ChessState
		wantErr    error
	}{
		{
			name:       "JoinWhite",
			gameID:     whiteGame.ID,
			joinPlayer: model.MakePlayer(1, "name", "us"),
			wantGame: mutateGame(whiteGame, func(s *ChessState) {
				s.WhitePlayer = model.MakePlayer(1, "name", "us")
			}),
		},
		{
			name:       "BothPlayersExist",
			gameID:     fullGame.ID,
			joinPlayer: model.PlayerState{ID: 3, Name: "testing", Present: true},
			wantGame:   fullGame,
		},
		{
			name:       "GameNotFound",
			gameID:     missingGameID,
			joinPlayer: model.MakePlayer(4, "ghost", "us"),
			wantErr:    ErrNoChessState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			updatedState, err := services.JoinGame(ctx, tt.gameID, tt.joinPlayer)

			assert.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantGame, updatedState, ChessMetaCmpOpt)
			AssertRedisChessState(t, services, tt.wantGame, ChessMetaCmpOpt)
		})
	}
}

func TestAttemptUndo(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	noMovesGame := MakeChessState(StateSetup{
		ID:         "test-undo-no-moves-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame := MakeChessState(StateSetup{
		ID:         "test-undo-with-moves-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame.Game.Moves = []hexchess.HistMove{{PieceMove: hexchess.PieceMove{Piece: 1, To: hexchess.Hex{Rank: 1}}}}
	missingGameID := "test-undo-missing-" + uuid.NewString()

	seedGames(t, services, noMovesGame, withMovesGame)

	type subTest struct {
		kind     UndoKind
		player   model.PlayerState
		wantGame *ChessState
		wantErr  error
	}

	tests := []struct {
		name   string
		gameID string
		tests  []subTest
	}{
		{
			name:   "TryingToUndoAnInvalidGame",
			gameID: missingGameID,
			tests: []subTest{
				{
					wantErr: ErrNoChessState,
				},
			},
		},
		{
			name:   "CreatingThenAcceptingAnUndo",
			gameID: withMovesGame.ID,
			tests: []subTest{
				{
					kind:   UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(withMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoAccept,
					player: model.PlayerState{ID: 1, Name: "white", Present: true},
					wantGame: mutateGame(withMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{}
					}),
				},
			},
		},
		{
			name:   "CreatingThenRejectingOwnUndo",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:   UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoReject,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{}
					}),
				},
			},
		},
		{
			name:   "CreatingThenRejectingUndo",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:   UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoReject,
					player: model.PlayerState{ID: 1, Name: "white", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{}
					}),
				},
			},
		},
		{
			name:   "RejectingAndAcceptingAnUndoWithoutCreating",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:    UndoReject,
					player:  model.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
				{
					kind:    UndoAccept,
					player:  model.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
			},
		},
		{
			name:   "CreatingAnUndoAcceptingAsCreatorThenAcceptingWithNoPreviousMoves",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:   UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:    UndoAccept,
					player:  model.PlayerState{ID: 2, Name: "black", Present: true},
					wantErr: ErrUndoNoop,
				},
				{
					kind:    UndoAccept,
					player:  model.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoMoveUndo,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, tt.name)

			for _, subTest := range tt.tests {

				updatedState, err := services.AttemptGameUndo(ctx, tt.gameID, subTest.player, subTest.kind)

				assert.Equal(t, subTest.wantErr, err)

				cmptOpts := cmpopts.IgnoreFields(ChessState{}, "Game", "Touch")
				testutil.Equal(t, subTest.wantGame, updatedState, cmptOpts)
				AssertRedisChessState(t, services, updatedState, cmptOpts)
			}
		})
	}
}

func TestMakeMove(t *testing.T) {
	t.Parallel()

	stateWhiteTurn := MakeChessState(StateSetup{
		ID:         "test-makemove-white-turn-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
	})
	stateEnded := MakeChessState(StateSetup{
		ID:         "test-makemove-ended-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	stateNotStarted := MakeChessState(StateSetup{
		ID:         "test-makemove-not-started-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
	})
	stateIntoCheckmate := MakeChessState(StateSetup{
		ID:         "test-makemove-checkmate-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 3, Present: true},
		Black:      model.PlayerState{ID: 4, Present: true},
		Game: ptr(hexchess.MakeEmptyGame(false,
			hexchess.Place{Not: "f1", Piece: hexchess.WhiteKing},
			hexchess.Place{Not: "a2", Piece: hexchess.BlackQueen},
			hexchess.Place{Not: "h1", Piece: hexchess.BlackRook},
			hexchess.Place{Not: "f3", Piece: hexchess.BlackRook},
			hexchess.Place{Not: "f9", Piece: hexchess.BlackKing},
		)),
	})
	missingGameID := "test-makemove-missing-" + uuid.NewString()

	stateWhiteMoved := mutateGame(stateWhiteTurn, func(s *ChessState) {
		s.Game.MakeMove(hexchess.MoveStr("b1", "b2"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []hexchess.HistMove{
			{PieceMove: hexchess.PieceMove{Piece: hexchess.WhitePawn, From: hexchess.HexStr("b1"), To: hexchess.HexStr("b2")}, Notation: "Pb2"},
		}
	})
	stateCheckmated := mutateGame(stateIntoCheckmate, func(s *ChessState) {
		s.Game.MakeMove(hexchess.MoveStr("a2", "a1"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []hexchess.HistMove{
			{PieceMove: hexchess.PieceMove{Piece: hexchess.BlackQueen, From: hexchess.HexStr("a2"), To: hexchess.HexStr("a1")}, Notation: "qa1"},
		}
		s.EndState = Finished
	})

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	seedGames(t, services, stateWhiteTurn, stateEnded, stateNotStarted, stateIntoCheckmate)

	tests := []struct {
		name     string
		move     hexchess.Move
		stateID  string
		player   model.PlayerState
		wantErr  error
		wantGame *ChessState
	}{
		{
			name:    "GameDoesNotExist",
			stateID: missingGameID,
			wantErr: ErrNoChessState,
		},
		{
			name:     "InvalidTurn",
			move:     hexchess.Move{To: hexchess.Hex{File: 1}},
			stateID:  stateWhiteTurn.ID,
			player:   stateWhiteTurn.BlackPlayer,
			wantErr:  ErrTurn{GameID: stateWhiteTurn.ID, PlayerID: 2, CurrID: 1},
			wantGame: stateWhiteTurn,
		},
		{
			name:     "InvalidEndedMove",
			move:     hexchess.Move{To: hexchess.Hex{File: 1}},
			stateID:  stateEnded.ID,
			player:   stateEnded.WhitePlayer,
			wantErr:  ErrFinishedGame{GameID: stateEnded.ID},
			wantGame: stateEnded,
		},
		{
			name:     "InvalidNotStartedMove",
			move:     hexchess.Move{To: hexchess.Hex{File: 1}},
			stateID:  stateNotStarted.ID,
			player:   stateNotStarted.WhitePlayer,
			wantErr:  ErrStartedGame{GameID: stateNotStarted.ID},
			wantGame: stateNotStarted,
		},
		{
			name:    "InvalidMove",
			move:    hexchess.Move{To: hexchess.Hex{File: 1}},
			stateID: stateWhiteTurn.ID,
			player:  stateWhiteTurn.WhitePlayer,
			wantErr: ErrInvalidMove{
				GameID:    stateWhiteTurn.ID,
				PlayerID:  1,
				Violation: hexchess.MoveError{Kind: hexchess.MoveErrIllegalTarget, Move: hexchess.Move{To: hexchess.Hex{File: 1}}},
			},
			wantGame: stateWhiteTurn,
		},
		{
			name:     "ValidMoveAsWhite",
			move:     hexchess.MoveStr("b1", "b2"),
			stateID:  stateWhiteTurn.ID,
			player:   stateWhiteTurn.WhitePlayer,
			wantGame: stateWhiteMoved,
		},
		{
			name:     "ValidMoveAsBlackIntoCheckmate",
			move:     hexchess.MoveStr("a2", "a1"),
			stateID:  stateIntoCheckmate.ID,
			player:   stateIntoCheckmate.BlackPlayer,
			wantGame: stateCheckmated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, tt.name)

			moveResult, err := services.MakeGameMove(ctx, tt.stateID, tt.player, tt.move)

			assert.Equal(t, tt.wantErr, err)
			AssertRedisChessState(t, services, tt.wantGame, ChessMetaCmpOpt)
			if tt.wantErr == nil {
				testutil.Equal(t, tt.wantGame, moveResult.State, ChessMetaCmpOpt)
			}
		})
	}
}

func TestForfeit(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	abortGame := MakeChessState(StateSetup{
		ID:         "test-forfeit-abort-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
	})
	forfeitGame := MakeChessState(StateSetup{
		ID:         "test-forfeit-full-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
	})
	forfeitGame.Game.Moves = []hexchess.HistMove{{
		PieceMove: hexchess.PieceMove{Piece: 1, To: hexchess.Hex{Rank: 1}},
	}}

	seedGames(t, services, abortGame, forfeitGame)

	t.Run("Abort", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		endState, err := services.EndGame(ctx, abortGame.ID, abortGame.WhitePlayer)
		require.NoError(t, err)
		assert.Equal(t, Aborted, endState)

		wantGame := mutateGame(abortGame, func(s *ChessState) {
			s.EndState = Aborted
		})
		AssertRedisChessState(t, services, wantGame, ChessMetaCmpOpt)

		gameKey := services.gameKey(abortGame.ID)
		zRankErr := services.redis.Cache.ZRank(ctx, services.redis.GamesZSet, gameKey).Err()
		assert.Equal(t, redis.Nil, zRankErr)
		zRankErr = services.redis.Cache.ZRank(ctx, services.userGameZSet(abortGame.WhitePlayer.ID), gameKey).Err()
		assert.Equal(t, redis.Nil, zRankErr)
	})

	t.Run("Forfeit", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

		endState, err := services.EndGame(ctx, forfeitGame.ID, forfeitGame.BlackPlayer)
		require.NoError(t, err)
		assert.Equal(t, Finished, endState)

		wantGame := mutateGame(forfeitGame, func(s *ChessState) {
			s.EndState = Finished
		})
		AssertRedisChessState(t, services, wantGame, ChessMetaCmpOpt)
	})
}

func TestForfeit_Errors(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.Redis)
	defer services.Close()

	endedGame := MakeChessState(StateSetup{
		ID:         "test-forfeit-err-ended-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	cannotAbortGame := MakeChessState(StateSetup{
		ID:         "test-forfeit-err-cannot-abort-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 2, Present: true},
	})
	notPlayerGame := MakeChessState(StateSetup{
		ID:         "test-forfeit-err-not-player-" + uuid.NewString(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 3, Present: true},
		Black:      model.PlayerState{ID: 4, Present: true},
	})
	missingGameID := "test-forfeit-err-missing-" + uuid.NewString()

	seedGames(t, services, endedGame, cannotAbortGame, notPlayerGame)

	tests := []struct {
		name    string
		gameID  string
		player  model.PlayerState
		wantErr error
	}{
		{
			name:    "GameDoesNotExist",
			gameID:  missingGameID,
			wantErr: ErrNoChessState,
		},
		{
			name:    "IsAlreadyEnded",
			gameID:  endedGame.ID,
			wantErr: ErrFinishedGame{GameID: endedGame.ID},
		},
		{
			name:    "CannotAbortWithoutBeingAPlayer",
			gameID:  cannotAbortGame.ID,
			wantErr: ErrForfeitPlayer,
		},
		{
			name:    "IsNotAPlayer",
			gameID:  notPlayerGame.ID,
			wantErr: ErrForfeitPlayer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

			_, forfeitErr := services.EndGame(ctx, tt.gameID, model.PlayerState{ID: 1, Present: true})

			assert.Equal(t, tt.wantErr, forfeitErr)
		})
	}
}
