package web

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"testing"
	"time"
)

func assertStateRdb(t *testing.T, rdb data.Redis, expState data.ChessState) {
	ctx := context.WithValue(context.Background(), lib.TK, "assert-chess-states")
	actualState, err := data.GetChessState(ctx, rdb, expState.ID)
	if err != nil {
		t.Fatalf("failed to get chess for assert: %v", err)
	}
	lib.AssertEqualIgnoring(t, expState, actualState, data.ChessMetaCmpOpts)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-join-game")

	gameID := "test123"
	inState := data.MakeState(gameID, data.RealTime)
	inState.FirstColor = data.White
	player := data.PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	if _, err := data.SetChessState(ctx, stores.Rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	updated, err := JoinGame(ctx, stores, gameID, player)

	expState := inState.DeepCopy()
	expState.WhitePlayer = &player

	assert.NoError(t, err)
	lib.AssertEqualIgnoring(t, expState, updated, data.ChessMetaCmpOpts)
	assertStateRdb(t, stores.Rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-join-game-both-players")

	gameID := "test123"
	inState := data.MakeState(gameID, data.RealTime)
	inState.WhitePlayer = &data.PlayerState{ID: 1, Name: "white"}
	inState.BlackPlayer = &data.PlayerState{ID: 2, Name: "black"}

	if _, err := data.SetChessState(ctx, stores.Rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	result, err := JoinGame(ctx, stores, gameID, data.PlayerState{ID: 3, Name: "test"})

	assert.NoError(t, err)
	lib.AssertEqualIgnoring(t, inState, result, data.ChessMetaCmpOpts)
	assertStateRdb(t, stores.Rdb, inState)
}

func TestMakeMove(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	s1 := data.MakeState("test1", data.RealTime)
	s1.WhitePlayer = &data.PlayerState{ID: 1}
	s1.BlackPlayer = &data.PlayerState{ID: 2}

	s2 := data.ChessState{
		Game: chess.MakeEmptyGame(),
		ChessMeta: data.ChessMeta{
			ID:          "test2",
			FirstColor:  data.Random,
			TimeControl: data.RealTime,
			Touch:       time.UnixMilli(0),
			WhitePlayer: &data.PlayerState{ID: 3},
			BlackPlayer: &data.PlayerState{ID: 4},
		},
	}
	s2.Game.Board.IsWhiteTurn = false
	s2.Game.
		SetPiece("f1", chess.WhiteKing).
		SetPiece("a2", chess.BlackQueen).
		SetPiece("h1", chess.BlackRook).
		SetPiece("f3", chess.BlackRook).
		SetPiece("f9", chess.BlackKing)

	ctx := context.WithValue(context.Background(), lib.TK, "testing-make-move")

	for _, state := range []data.ChessState{s1, s2} {
		if _, err := data.SetChessState(ctx, stores.Rdb, state.ID, state); err != nil {
			t.Fatalf("failed initialize test state: %v", err)
		}
	}

	for i, test := range []struct {
		pm     chess.PieceMove
		state  data.ChessState
		player data.PlayerState
		expErr error
	}{
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid turn
			state:  s1,
			player: *s1.BlackPlayer,
			expErr: ErrTurn,
		},
		{
			pm:     chess.PieceMove{To: chess.Hex{File: 1}}, // invalid move
			state:  s1,
			player: *s1.WhitePlayer,
			expErr: ErrInvalidMove,
		},
		{
			pm:     chess.PieceMove{Piece: chess.WhitePawn, From: chess.Hex{File: 1, Rank: 0}, To: chess.Hex{File: 1, Rank: 1}}, // valid move
			state:  s1,
			player: *s1.WhitePlayer,
		},
		{
			pm:     chess.PieceMove{Piece: chess.BlackQueen, From: chess.Hex{File: 0, Rank: 1}, To: chess.Hex{File: 0, Rank: 0}}, // valid move
			state:  s2,
			player: *s2.BlackPlayer,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			result, err := MakeGameMove(ctx, stores, test.state.ID, test.player, test.pm)
			if err != nil {
				assert.Equal(t, test.expErr, err)
			} else {
				assert.Equal(t, test.pm, result.Move)
			}
		})
	}
}

func TestForfeit_BlackForfeits(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-forfeit")

	gameID := "test123"
	inState := data.MakeState(gameID, data.RealTime)
	inState.WhitePlayer = &data.PlayerState{ID: 1}
	inState.BlackPlayer = &data.PlayerState{ID: 2}
	inState.MoveList = []chess.PieceMove{{Piece: 1, To: chess.Hex{Rank: 1}}}

	if _, err := data.SetChessState(ctx, stores.Rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	assert.NoError(t, ForfeitGame(ctx, stores, gameID, *inState.BlackPlayer))

	expState := inState.DeepCopy()
	expState.IsEnded = true

	assertStateRdb(t, stores.Rdb, expState)
}
