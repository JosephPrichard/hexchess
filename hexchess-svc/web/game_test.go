package web

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/chess"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"strconv"
	"testing"
	"time"
)

func assertStateRdb(t *testing.T, rdb data.Redis, expState data.ChessState) {
	ctx := context.WithValue(context.Background(), util.Trace, "assert-chess-states")
	actualState, err := data.GetChessState(ctx, rdb, expState.ID)
	if err != nil {
		t.Fatalf("failed to get chess for assert: %v", err)
	}
	util.AssertEqualIgnoring(t, expState, actualState, data.ChessMetaCmpOpts)
}

func TestJoinGame_JoinWhite(t *testing.T) {
	rdb := data.BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game")

	gameID := "test123"
	inState := data.MakeState(gameID, data.RealTime)
	inState.FirstColor = data.White
	player := data.PlayerState{ID: 1, Name: "name", Country: "us", Elo: 0}

	if _, err := data.SetChessState(ctx, rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	updated, err := JoinGame(ctx, rdb, gameID, player)
	assert.NoError(t, err)

	expState := inState.DeepCopy()
	expState.WhitePlayer = &player

	util.AssertEqualIgnoring(t, expState, updated, data.ChessMetaCmpOpts)
	assertStateRdb(t, rdb, updated)
}

func TestJoinGame_BothPlayersExist(t *testing.T) {
	rdb := data.BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-join-game-both-players")

	gameID := "test123"
	inState := data.MakeState(gameID, data.RealTime)
	inState.WhitePlayer = &data.PlayerState{ID: 1, Name: "white"}
	inState.BlackPlayer = &data.PlayerState{ID: 2, Name: "black"}

	if _, err := data.SetChessState(ctx, rdb, gameID, inState); err != nil {
		t.Fatalf("failed initialize test state: %v", err)
	}

	result, err := JoinGame(ctx, rdb, gameID, data.PlayerState{ID: 3, Name: "test"})

	assert.NoError(t, err)
	util.AssertEqualIgnoring(t, inState, result, data.ChessMetaCmpOpts)
	assertStateRdb(t, rdb, inState)
}

func TestMakeMove(t *testing.T) {
	stores, closer := data.BeforeStoresTests(t, true)
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

	ctx := context.WithValue(context.Background(), util.Trace, "testing-make-move")

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
		t.Run(strconv.Itoa(i), func(t *testing.T) {
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
	stores, closer := data.BeforeStoresTests(t, true)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-forfeit")

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
