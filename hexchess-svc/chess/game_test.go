package chess

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHexagon_String(t *testing.T) {
	assert.Equal(t, "f9", Hex{File: 5, Rank: 8}.String())
	assert.Equal(t, "g10", Hex{File: 6, Rank: 9}.String())
	assert.Equal(t, "e5", Hex{File: 4, Rank: 4}.String())
	assert.Equal(t, "g1", Hex{File: 6, Rank: 0}.String())
}

func TestParseHexagon(t *testing.T) {
	assert.Equal(t, Hex{File: 5, Rank: 8}, ParseHexagonValid("f9"))
	assert.Equal(t, Hex{File: 6, Rank: 9}, ParseHexagonValid("g10"))
	assert.Equal(t, Hex{File: 4, Rank: 4}, ParseHexagonValid("e5"))
}

func TestGame_GetSetPieces(t *testing.T) {
	board := InitialBoard()
	board.SetPieceNot("f3", WhiteBishop)
	piece := board.GetPieceNot("f3")
	assert.Equal(t, WhiteBishop, piece)
}

func TestGame_DetermineIsCheckmate(t *testing.T) {
	game1 := MakeEmptyGame()
	game1.
		SetPiece("f6", WhiteKing).
		SetPiece("f4", BlackQueen).
		SetPiece("f8", BlackQueen).
		SetPiece("b4", BlackBishop).
		SetPiece("j4", BlackBishop).
		SetPiece("f9", BlackKing)
	game2 := MakeEmptyGame()
	game2.
		SetPiece("f1", WhiteKing).
		SetPiece("a1", BlackQueen).
		SetPiece("h1", BlackRook).
		SetPiece("f3", BlackRook).
		SetPiece("f9", BlackKing)

	for i, game := range []Game{game1, game2} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			game.InitPieceMoves()
			t.Logf("game:\n%s", game.StringColor(!game.Board.IsWhiteTurn))
			isCheckmate := game.CheckmateReached()
			assert.True(t, isCheckmate)
		})
	}
}

func assertMoves(t *testing.T, actual []Hex, expected ...string) {
	var expectedMoves []Hex
	for _, s := range expected {
		expectedMoves = append(expectedMoves, ParseHexagonValid(s))
	}
	assert.ElementsMatch(t, expectedMoves, actual)
}

func TestGame_FindMoves(t *testing.T) {
	for _, test := range []struct {
		name     string
		game     Game
		hex      Hex
		f        func(*Game, Hex) PieceMoves
		expMoves []string
	}{
		{
			name:     "TestCenterRook",
			game:     MakeStartGame(Move{"f6", BlackRook}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).FindRookMoves,
			expMoves: []string{"f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1", "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6"},
		},
		{
			name:     "RookLeft",
			game:     MakeStartGame(Move{"c8", BlackRook}),
			hex:      Hex{File: 2, Rank: 7},
			f:        (*Game).FindRookMoves,
			expMoves: []string{"d8", "e8", "f8"},
		},
		{
			name:     "RookRight",
			game:     MakeStartGame(Move{"h4", BlackRook}),
			hex:      Hex{File: 7, Rank: 3},
			f:        (*Game).FindRookMoves,
			expMoves: []string{"h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6", "d6", "c6", "b6", "a6", "i4", "j4", "k4"},
		},
		{
			name:     "BishopCenter",
			game:     MakeStartGame(Move{"f6", BlackBishop}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).FindBishopMoves,
			expMoves: []string{"h5", "j4", "d5", "b4", "g4", "e4"},
		},
		{
			name:     "BishopLeft",
			game:     MakeStartGame(Move{"c8", BlackBishop}),
			hex:      Hex{File: 2, Rank: 7},
			f:        (*Game).FindBishopMoves,
			expMoves: []string{"e9", "g9", "b6", "a4"},
		},
		{
			name:     "BishopRight",
			game:     MakeStartGame(Move{"h4", BlackBishop}),
			hex:      Hex{File: 7, Rank: 3},
			f:        (*Game).FindBishopMoves,
			expMoves: []string{"j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2"},
		},
		{
			name:     "KingCenter",
			game:     MakeEmptyGame(Move{"f6", WhiteKing}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).FindKingMoves,
			expMoves: []string{"f7", "f5", "e5", "g5", "e6", "g6", "h5", "d5", "g7", "e7", "g4", "e4"},
		},
		{
			name:     "KingLeft",
			game:     MakeEmptyGame(Move{"d3", WhiteKing}),
			hex:      Hex{File: 3, Rank: 2},
			f:        (*Game).FindKingMoves,
			expMoves: []string{"d4", "d2", "c2", "e3", "c3", "e4", "f4", "b2", "e5", "c4", "e2", "c1"},
		},
		{
			name:     "KingRight",
			game:     MakeEmptyGame(Move{"h7", WhiteKing}),
			hex:      Hex{File: 7, Rank: 6},
			f:        (*Game).FindKingMoves,
			expMoves: []string{"h8", "h6", "g7", "i6", "g8", "i7", "j6", "f8", "i8", "g9", "i5", "g6"},
		},
		{
			name:     "KnightCenter",
			game:     MakeEmptyGame(Move{"f6", WhiteKnight}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).FindKnightMoves,
			expMoves: []string{"h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4"},
		},
		{
			name:     "KnightLeft",
			game:     MakeEmptyGame(Move{"d3", WhiteKnight}),
			hex:      Hex{File: 3, Rank: 2},
			f:        (*Game).FindKnightMoves,
			expMoves: []string{"f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3"},
		},
		{
			name:     "KnightRight",
			game:     MakeEmptyGame(Move{"h7", WhiteKnight}),
			hex:      Hex{File: 7, Rank: 6},
			f:        (*Game).FindKnightMoves,
			expMoves: []string{"j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5"},
		},
		{
			name:     "PawnFirstMove",
			game:     MakeStartGame(Move{"g4", WhitePawn}),
			hex:      Hex{File: 6, Rank: 3},
			f:        (*Game).FindPawnMovesWhite,
			expMoves: []string{"g5", "g6"},
		},
		{
			name: "PawnTakeMove",
			game: MakeStartGame(
				Move{"c4", BlackKnight},
				Move{"e5", WhiteKnight},
				Move{"d5", BlackPawn},
			),
			hex:      Hex{File: 3, Rank: 4},
			f:        (*Game).FindPawnMovesBlack,
			expMoves: []string{"d4", "e5"},
		},
	} {
		t.Run(fmt.Sprintf("%s", test.name), func(t *testing.T) {
			t.Logf("testing moves for piece at: %v on game:%s", test.hex, test.game.Board.String())
			moves := test.f(&test.game, test.hex).Moves
			assertMoves(t, moves, test.expMoves...)
		})
	}
}
