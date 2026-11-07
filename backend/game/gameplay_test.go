package game

import (
	"context"
	"hexchess-svc/chess"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/itest"
	"hexchess-svc/player"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameplayTest(t slogutil.TestLogger) (*GamePlayService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewGameplayService(
		infra.Redis,
		NewChessRepoService(infra.Redis),
	)

	return service, infra
}

func TestJoinGame(t *testing.T) {
	service, testinfra := setupGameplayTest(t)
	defer testinfra.Close()

	whiteGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.White,
	})
	fullGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      player.PlayerState{ID: 2, Name: "black", Present: true},
	})
	missingGameID := gameid.NewGameID()

	setChessStates(t, testinfra.Redis, whiteGame, fullGame)

	tests := []struct {
		name       string
		gameID     gameid.Identifier
		joinPlayer player.PlayerState
		wantGame   *ChessState
		wantErr    error
	}{
		{
			name:       "JoinWhite",
			gameID:     whiteGame.ID,
			joinPlayer: player.NewPlayer(1, "name", "us"),
			wantGame: mutateGame(whiteGame, func(s *ChessState) {
				s.WhitePlayer = player.NewPlayer(1, "name", "us")
			}),
		},
		{
			name:       "BothPlayersExist",
			gameID:     fullGame.ID,
			joinPlayer: player.PlayerState{ID: 3, Name: "testing", Present: true},
			wantGame:   fullGame,
		},
		{
			name:       "GameNotFound",
			gameID:     missingGameID,
			joinPlayer: player.NewPlayer(4, "ghost", "us"),
			wantErr:    ErrNoChessState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			updatedState, err := service.JoinGame(ctx, tt.gameID, tt.joinPlayer)

			assert.Equal(t, tt.wantErr, err)
			testutil.Equal(t, tt.wantGame, updatedState, cmpOptsChessState)
			assertRedisChess(t, testinfra.Redis, tt.wantGame, cmpOptsChessState)
		})
	}
}

func TestAttemptUndo(t *testing.T) {
	service, testinfra := setupGameplayTest(t)
	defer testinfra.Close()

	noMovesGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      player.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Name: "white", Present: true},
		Black:      player.PlayerState{ID: 2, Name: "black", Present: true},
	})
	withMovesGame.Game.Moves = []chess.HistMove{{PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}}}}
	missingGameID := gameid.NewGameID()

	setChessStates(t, testinfra.Redis, noMovesGame, withMovesGame)

	type subTest struct {
		kind     UndoKind
		player   player.PlayerState
		wantGame *ChessState
		wantErr  error
	}

	tests := []struct {
		name   string
		gameID gameid.Identifier
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
					player: player.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(withMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoAccept,
					player: player.PlayerState{ID: 1, Name: "white", Present: true},
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
					player: player.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoReject,
					player: player.PlayerState{ID: 2, Name: "black", Present: true},
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
					player: player.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:   UndoReject,
					player: player.PlayerState{ID: 1, Name: "white", Present: true},
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
					player:  player.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoUndo,
				},
				{
					kind:    UndoAccept,
					player:  player.PlayerState{ID: 1, Name: "white", Present: true},
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
					player: player.PlayerState{ID: 2, Name: "black", Present: true},
					wantGame: mutateGame(noMovesGame, func(s *ChessState) {
						s.UndoState = UndoState{UndoID: 2}
					}),
				},
				{
					kind:    UndoAccept,
					player:  player.PlayerState{ID: 2, Name: "black", Present: true},
					wantErr: ErrUndoNoop,
				},
				{
					kind:    UndoAccept,
					player:  player.PlayerState{ID: 1, Name: "white", Present: true},
					wantErr: ErrNoMoveUndo,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(t.Context(), slogutil.Trace, tt.name)

			for _, subTest := range tt.tests {

				updatedState, err := service.AttemptGameUndo(ctx, tt.gameID, subTest.player, subTest.kind)

				assert.Equal(t, subTest.wantErr, err)

				cmpOpts := cmpopts.IgnoreFields(ChessState{}, "Game", "StartTime")
				testutil.Equal(t, subTest.wantGame, updatedState, cmpOpts)
				assertRedisChess(t, testinfra.Redis, updatedState, cmpOpts)
			}
		})
	}
}

func TestNewMove(t *testing.T) {
	stateWhiteTurn := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
		Black:      player.PlayerState{ID: 2, Present: true},
	})
	stateEnded := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
		Black:      player.PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	stateNotStarted := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
	})
	stateIntoCheckmate := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 3, Present: true},
		Black:      player.PlayerState{ID: 4, Present: true},
		Game: new(chess.NewEmptyGame(false,
			chess.Place{Not: "f1", Piece: chess.WhiteKing},
			chess.Place{Not: "a2", Piece: chess.BlackQueen},
			chess.Place{Not: "h1", Piece: chess.BlackRook},
			chess.Place{Not: "f3", Piece: chess.BlackRook},
			chess.Place{Not: "f9", Piece: chess.BlackKing},
		)),
	})
	missingGameID := gameid.NewGameID()

	stateWhiteMoved := mutateGame(stateWhiteTurn, func(s *ChessState) {
		s.Game.NewMove(chess.MoveStr("b1", "b2"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []chess.HistMove{
			{PieceMove: chess.PieceMove{Piece: chess.WhitePawn, From: chess.HexStr("b1"), To: chess.HexStr("b2")}, Notation: "Pb2"},
		}
	})
	stateCheckmated := mutateGame(stateIntoCheckmate, func(s *ChessState) {
		s.Game.NewMove(chess.MoveStr("a2", "a1"))
		s.Game.InitPieceMoves()
		s.Game.ClearTables()
		s.Game.Moves = []chess.HistMove{
			{PieceMove: chess.PieceMove{Piece: chess.BlackQueen, From: chess.HexStr("a2"), To: chess.HexStr("a1")}, Notation: "qa1"},
		}
		s.EndState = Finished
	})

	service, testinfra := setupGameplayTest(t)
	defer testinfra.Close()

	setChessStates(t, testinfra.Redis, stateWhiteTurn, stateEnded, stateNotStarted, stateIntoCheckmate)

	tests := []struct {
		name     string
		move     chess.Move
		stateID  gameid.Identifier
		player   player.PlayerState
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
			ctx := context.WithValue(t.Context(), slogutil.Trace, tt.name)

			moveResult, err := service.MakeGameMove(ctx, tt.stateID, tt.player, tt.move)

			assert.Equal(t, tt.wantErr, err)
			assertRedisChess(t, testinfra.Redis, tt.wantGame, cmpOptsChessState)
			if tt.wantErr == nil {
				testutil.Equal(t, tt.wantGame, moveResult.State, cmpOptsChessState)
			}
		})
	}
}

func TestForfeit(t *testing.T) {
	service, testinfra := setupGameplayTest(t)
	defer testinfra.Close()

	abortGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
	})
	forfeitGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
		Black:      player.PlayerState{ID: 2, Present: true},
	})
	forfeitGame.Game.Moves = []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: 1, To: chess.Hex{Rank: 1}},
	}}

	setChessStates(t, testinfra.Redis, abortGame, forfeitGame)

	t.Run("Abort", func(t *testing.T) {
		ctx := t.Context()

		endState, err := service.EndGame(ctx, abortGame.ID, abortGame.WhitePlayer)
		require.NoError(t, err)
		assert.Equal(t, Aborted, endState)

		wantGame := mutateGame(abortGame, func(s *ChessState) {
			s.EndState = Aborted
		})
		assertRedisChess(t, testinfra.Redis, wantGame, cmpOptsChessState)
	})

	t.Run("Forfeit", func(t *testing.T) {
		ctx := t.Context()

		endState, err := service.EndGame(ctx, forfeitGame.ID, forfeitGame.BlackPlayer)
		require.NoError(t, err)
		assert.Equal(t, Finished, endState)

		wantGame := mutateGame(forfeitGame, func(s *ChessState) {
			s.EndState = Finished
		})
		assertRedisChess(t, testinfra.Redis, wantGame, cmpOptsChessState)
	})
}

func TestForfeit_Errors(t *testing.T) {
	service, testinfra := setupGameplayTest(t)
	defer testinfra.Close()

	endedGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 1, Present: true},
		Black:      player.PlayerState{ID: 2, Present: true},
		EndState:   Finished,
	})
	cannotAbortGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 2, Present: true},
	})
	notPlayerGame := NewChessState(ChessStateArgs{
		ID:         gameid.NewGameID(),
		Mode:       gamedata.ModeCorrespondence1,
		FirstColor: gamedata.Random,
		White:      player.PlayerState{ID: 3, Present: true},
		Black:      player.PlayerState{ID: 4, Present: true},
	})
	missingGameID := gameid.NewGameID()

	setChessStates(t, testinfra.Redis, endedGame, cannotAbortGame, notPlayerGame)

	tests := []struct {
		name    string
		gameID  gameid.Identifier
		player  player.PlayerState
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
			ctx := t.Context()

			_, forfeitErr := service.EndGame(ctx, tt.gameID, player.PlayerState{ID: 1, Present: true})

			assert.Equal(t, tt.wantErr, forfeitErr)
		})
	}
}
