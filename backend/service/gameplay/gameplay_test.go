package gameplay

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/gamestate"

	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameplayTest(t alog.TestLogger, flags ...itest.TestFlag) (*GamePlayService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewGameplayService(
		infra.Redis,
		producers.NewStreamProducer(infra.Redis),
		gamestate.NewChessRepoService(infra.Redis),
	)

	return services, infra
}

func assertRedisChess(t *testing.T, redis cache.Redis, wantState *model.ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := t.Context()

	if wantState == nil {
		return
	}
	actualState, err := gamestate.NewChessRepoService(redis).GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("failed to retrieve in redis chess state assert: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}

func mutateGame(state *model.ChessState, fn func(s *model.ChessState)) *model.ChessState {
	s := state.DeepCopy()
	fn(&s)
	return &s
}

func setChessStates(t *testing.T, redis cache.Redis, games ...*model.ChessState) {
	for _, g := range games {
		require.NoError(t, gamestate.NewChessRepoService(redis).SetChessState(t.Context(), g.ID, g))
	}
}

func TestJoinGame(t *testing.T) {
	services, testinfra := setupGameplayTest(t, itest.Redis)
	defer testinfra.Close()

	whiteGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.White,
	})
	fullGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	missingGameID := model.NewGameID()

	setChessStates(t, testinfra.Redis, whiteGame, fullGame)

	tests := []struct {
		name       string
		gameID     model.GameID
		joinPlayer model.PlayerState
		wantGame   *model.ChessState
		wantErr    error
	}{
		{
			name:       "JoinWhite",
			gameID:     whiteGame.ID,
			joinPlayer: model.NewPlayer(1, "name", "us"),
			wantGame: mutateGame(whiteGame, func(s *model.ChessState) {
				s.WhitePlayer = model.NewPlayer(1, "name", "us")
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
			joinPlayer: model.NewPlayer(4, "ghost", "us"),
			wantErr:    gamestate.ErrNoChessState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			updatedState, err := services.JoinGame(ctx, tt.gameID, tt.joinPlayer)

			assert.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantGame, updatedState)
			assertRedisChess(t, testinfra.Redis, tt.wantGame)
		})
	}
}

func TestAttemptUndo(t *testing.T) {
	services, testinfra := setupGameplayTest(t, itest.Redis)
	defer testinfra.Close()

	noMovesGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      model.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame.Game.Moves = []chess.HistMove{{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}}}
	missingGameID := model.NewGameID()

	setChessStates(t, testinfra.Redis, noMovesGame, withMovesGame)

	type subTest struct {
		kind     model.UndoKind
		player   model.PlayerState
		wantGame *model.ChessState
		wantErr  error
	}

	tests := []struct {
		name   string
		gameID model.GameID
		tests  []subTest
	}{
		{
			name:   "TryingToUndoAnInvalidGame",
			gameID: missingGameID,
			tests: []subTest{
				{
					wantErr: gamestate.ErrNoChessState,
				},
			},
		},
		{
			name:   "CreatingThenAcceptingAnUndo",
			gameID: withMovesGame.ID,
			tests: []subTest{
				{
					kind:   model.UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(withMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{UndoID: 2}
					}),
				},
				{
					kind:   model.UndoAccept,
					player: model.PlayerState{ID: 1, Name: "white", Present: true},
					wantGame: mutateGame(withMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{}
					}),
				},
			},
		},
		{
			name:   "CreatingThenRejectingOwnUndo",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:   model.UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{UndoID: 2}
					}),
				},
				{
					kind:   model.UndoReject,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{}
					}),
				},
			},
		},
		{
			name:   "CreatingThenRejectingUndo",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:   model.UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{UndoID: 2}
					}),
				},
				{
					kind:   model.UndoReject,
					player: model.PlayerState{ID: 1, Name: "white", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{}
					}),
				},
			},
		},
		{
			name:   "RejectingAndAcceptingAnUndoWithoutCreating",
			gameID: noMovesGame.ID,
			tests: []subTest{
				{
					kind:    model.UndoReject,
					player:  model.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
				{
					kind:    model.UndoAccept,
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
					kind:   model.UndoCreate,
					player: model.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *model.ChessState) {
						s.UndoState = model.UndoState{UndoID: 2}
					}),
				},
				{
					kind:    model.UndoAccept,
					player:  model.PlayerState{ID: 2, Name: "black", Present: true},
					wantErr: ErrUndoNoop,
				},
				{
					kind:    model.UndoAccept,
					player:  model.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: model.ErrNoMoveUndo,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), alog.Trace, tt.name)

			for _, subTest := range tt.tests {

				updatedState, err := services.AttemptGameUndo(ctx, tt.gameID, subTest.player, subTest.kind)

				assert.Equal(t, subTest.wantErr, err)

				cmptOpts := cmpopts.IgnoreFields(model.ChessState{}, "Game")
				testutil.Equal(t, subTest.wantGame, updatedState, cmptOpts)
				assertRedisChess(t, testinfra.Redis, updatedState, cmptOpts)
			}
		})
	}
}

func TestNewMove(t *testing.T) {
	stateWhiteTurn := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
	})
	stateEnded := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
		EndState:   model.Finished,
	})
	stateNotStarted := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
	})
	stateIntoCheckmate := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 3, Present: true},
		Black:      model.PlayerState{ID: 4, Present: true},
		Game: new(chess.NewEmptyGame(false,
			chess.Place{Not: "f1", Piece: chess.WhiteKing},
			chess.Place{Not: "a2", Piece: chess.BlackQueen},
			chess.Place{Not: "h1", Piece: chess.BlackRook},
			chess.Place{Not: "f3", Piece: chess.BlackRook},
			chess.Place{Not: "f9", Piece: chess.BlackKing},
		)),
	})
	missingGameID := model.NewGameID()

	stateWhiteMoved := mutateGame(stateWhiteTurn, func(s *model.ChessState) {
		s.Game.NewMove(chess.MoveStr("b1", "b2"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []chess.HistMove{
			{PieceMove: chess.PieceMove{Piece: chess.WhitePawn, From: chess.HexStr("b1"), To: chess.HexStr("b2")}, Notation: "Pb2"},
		}
	})
	stateCheckmated := mutateGame(stateIntoCheckmate, func(s *model.ChessState) {
		s.Game.NewMove(chess.MoveStr("a2", "a1"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []chess.HistMove{
			{PieceMove: chess.PieceMove{Piece: chess.BlackQueen, From: chess.HexStr("a2"), To: chess.HexStr("a1")}, Notation: "qa1"},
		}
		s.EndState = model.Finished
	})

	services, testinfra := setupGameplayTest(t, itest.Redis)
	defer testinfra.Close()

	setChessStates(t, testinfra.Redis, stateWhiteTurn, stateEnded, stateNotStarted, stateIntoCheckmate)

	tests := []struct {
		name     string
		move     chess.Move
		stateID  model.GameID
		player   model.PlayerState
		wantErr  error
		wantGame *model.ChessState
	}{
		{
			name:    "GameDoesNotExist",
			stateID: missingGameID,
			wantErr: gamestate.ErrNoChessState,
		},
		{
			name:     "InvalidTurn",
			move:     chess.Move{To: chess.Hex{File: 1}},
			stateID:  stateWhiteTurn.ID,
			player:   stateWhiteTurn.BlackPlayer,
			wantErr:  ErrTurn{GameID: stateWhiteTurn.ID, PlayerID: 2, CurrID: 1},
			wantGame: stateWhiteTurn,
		},
		{
			name:     "InvalidEndedMove",
			move:     chess.Move{To: chess.Hex{File: 1}},
			stateID:  stateEnded.ID,
			player:   stateEnded.WhitePlayer,
			wantErr:  ErrFinishedGame{GameID: stateEnded.ID},
			wantGame: stateEnded,
		},
		{
			name:     "InvalidNotStartedMove",
			move:     chess.Move{To: chess.Hex{File: 1}},
			stateID:  stateNotStarted.ID,
			player:   stateNotStarted.WhitePlayer,
			wantErr:  ErrStartedGame{GameID: stateNotStarted.ID},
			wantGame: stateNotStarted,
		},
		{
			name:    "InvalidMove",
			move:    chess.Move{To: chess.Hex{File: 1}},
			stateID: stateWhiteTurn.ID,
			player:  stateWhiteTurn.WhitePlayer,
			wantErr: ErrInvalidMove{
				GameID:    stateWhiteTurn.ID,
				PlayerID:  1,
				Violation: chess.MoveError{Kind: chess.MoveErrIllegalTarget, Move: chess.Move{To: chess.Hex{File: 1}}},
			},
			wantGame: stateWhiteTurn,
		},
		{
			name:     "ValidMoveAsWhite",
			move:     chess.MoveStr("b1", "b2"),
			stateID:  stateWhiteTurn.ID,
			player:   stateWhiteTurn.WhitePlayer,
			wantGame: stateWhiteMoved,
		},
		{
			name:     "ValidMoveAsBlackIntoCheckmate",
			move:     chess.MoveStr("a2", "a1"),
			stateID:  stateIntoCheckmate.ID,
			player:   stateIntoCheckmate.BlackPlayer,
			wantGame: stateCheckmated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), alog.Trace, tt.name)

			moveResult, err := services.NewGameMove(ctx, tt.stateID, tt.player, tt.move)

			assert.Equal(t, tt.wantErr, err)
			assertRedisChess(t, testinfra.Redis, tt.wantGame)
			if tt.wantErr == nil {
				testutil.Equal(t, tt.wantGame, moveResult.State)
			}
		})
	}
}

func TestForfeit(t *testing.T) {
	services, testinfra := setupGameplayTest(t, itest.Redis)
	defer testinfra.Close()

	abortGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
	})
	forfeitGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
	})
	forfeitGame.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	setChessStates(t, testinfra.Redis, abortGame, forfeitGame)

	t.Run("Abort", func(t *testing.T) {
		ctx := t.Context()

		endState, err := services.EndGame(ctx, abortGame.ID, abortGame.WhitePlayer)
		require.NoError(t, err)
		assert.Equal(t, model.Aborted, endState)

		wantGame := mutateGame(abortGame, func(s *model.ChessState) {
			s.EndState = model.Aborted
		})
		assertRedisChess(t, testinfra.Redis, wantGame)
	})

	t.Run("Forfeit", func(t *testing.T) {
		ctx := t.Context()

		endState, err := services.EndGame(ctx, forfeitGame.ID, forfeitGame.BlackPlayer)
		require.NoError(t, err)
		assert.Equal(t, model.Finished, endState)

		wantGame := mutateGame(forfeitGame, func(s *model.ChessState) {
			s.EndState = model.Finished
		})
		assertRedisChess(t, testinfra.Redis, wantGame)
	})
}

func TestForfeit_Errors(t *testing.T) {
	services, testinfra := setupGameplayTest(t, itest.Redis)
	defer testinfra.Close()

	endedGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 1, Present: true},
		Black:      model.PlayerState{ID: 2, Present: true},
		EndState:   model.Finished,
	})
	cannotAbortGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 2, Present: true},
	})
	notPlayerGame := model.NewChessState(model.StateSetup{
		ID:         model.NewGameID(),
		Mode:       model.ModeCorrespondence1,
		FirstColor: model.Random,
		White:      model.PlayerState{ID: 3, Present: true},
		Black:      model.PlayerState{ID: 4, Present: true},
	})
	missingGameID := model.NewGameID()

	setChessStates(t, testinfra.Redis, endedGame, cannotAbortGame, notPlayerGame)

	tests := []struct {
		name    string
		gameID  model.GameID
		player  model.PlayerState
		wantErr error
	}{
		{
			name:    "GameDoesNotExist",
			gameID:  missingGameID,
			wantErr: gamestate.ErrNoChessState,
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
			ctx := t.Context()

			_, forfeitErr := services.EndGame(ctx, tt.gameID, model.PlayerState{ID: 1, Present: true})

			assert.Equal(t, tt.wantErr, forfeitErr)
		})
	}
}
