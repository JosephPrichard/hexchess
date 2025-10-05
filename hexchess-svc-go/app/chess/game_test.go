package chess

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHexagonToString(t *testing.T) {
	assert.Equal(t, "f9", Hexagon{File: 5, Rank: 8}.String())
	assert.Equal(t, "g10", Hexagon{File: 6, Rank: 9}.String())
	assert.Equal(t, "e5", Hexagon{File: 4, Rank: 4}.String())
	assert.Equal(t, "g1", Hexagon{File: 6, Rank: 0}.String())
}

func TestHexagonFromString(t *testing.T) {
	assert.Equal(t, Hexagon{File: 5, Rank: 8}, ParseHexagonValid("f9"))
	assert.Equal(t, Hexagon{File: 6, Rank: 9}, ParseHexagonValid("g10"))
	assert.Equal(t, Hexagon{File: 4, Rank: 4}, ParseHexagonValid("e5"))
}

func TestGetSetPieces(t *testing.T) {
	board := InitialBoard()
	board.SetPiece("f3", WhiteBishop)
	piece := board.GetPiece("f3")
	assert.Equal(t, WhiteBishop, piece)
}

func TestDetermineIsCheckmate(t *testing.T) {
	game := EmptyGame()
	game.
		SetPiece("f6", WhiteKing).
		SetPiece("f4", BlackQueen).
		SetPiece("f8", BlackQueen).
		SetPiece("b4", BlackBishop).
		SetPiece("j4", BlackBishop).
		SetPiece("f9", BlackKing)

	t.Logf("game:\n%s", game.Board.String())
	game.InitPieceMoves()

	isCheckmate := game.CheckmateReached()
	assert.True(t, isCheckmate)
}

func assertMoves(t *testing.T, actual []Hexagon, expected ...string) {
	var expectedMoves []Hexagon
	for _, s := range expected {
		expectedMoves = append(expectedMoves, ParseHexagonValid(s))
	}
	assert.ElementsMatch(t, expectedMoves, actual)
}

func TestFindRookMoves(t *testing.T) {
	game1 := StartGame()
	game1.SetPiece("f6", BlackRook)
	game2 := StartGame()
	game2.SetPiece("c8", BlackRook)
	game3 := StartGame()
	game3.SetPiece("h4", BlackRook)

	centerMoves := game1.FindRookMoves(Hexagon{File: 5, Rank: 5}).Moves
	leftMoves := game2.FindRookMoves(Hexagon{File: 2, Rank: 7}).Moves
	rightMoves := game3.FindRookMoves(Hexagon{File: 7, Rank: 3}).Moves

	assertMoves(t, centerMoves,
		"f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1", "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6")

	assertMoves(t, leftMoves, "d8", "e8", "f8")

	assertMoves(t, rightMoves,
		"h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6", "d6", "c6", "b6", "a6", "i4", "j4", "k4")
}

func TestFindBishopMoves(t *testing.T) {
	game1 := StartGame()
	game1.SetPiece("f6", BlackBishop)
	game2 := StartGame()
	game2.SetPiece("c8", BlackBishop)
	game3 := StartGame()
	game3.SetPiece("h4", BlackBishop)

	centerMoves := game1.FindBishopMoves(Hexagon{File: 5, Rank: 5}).Moves
	leftMoves := game2.FindBishopMoves(Hexagon{File: 2, Rank: 7}).Moves
	rightMoves := game3.FindBishopMoves(Hexagon{File: 7, Rank: 3}).Moves

	assertMoves(t, centerMoves,
		"h5", "j4", "d5", "b4", "g4", "e4")

	assertMoves(t, leftMoves,
		"e9", "g9", "b6", "a4")

	assertMoves(t, rightMoves,
		"j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2")
}

func TestFindKingMoves(t *testing.T) {
	game1 := EmptyGame()
	game1.SetPiece("f6", WhiteKing)
	game2 := EmptyGame()
	game2.SetPiece("d3", WhiteKing)
	game3 := EmptyGame()
	game3.SetPiece("h7", WhiteKing)

	centerMoves := game1.FindKingMoves(Hexagon{File: 5, Rank: 5})
	leftMoves := game2.FindKingMoves(Hexagon{File: 3, Rank: 2})
	rightMoves := game3.FindKingMoves(Hexagon{File: 7, Rank: 6})

	assertMoves(t, centerMoves,
		"f7", "f5", "e5", "g5", "e6", "g6", "h5", "d5", "g7", "e7", "g4", "e4")

	assertMoves(t, leftMoves,
		"d4", "d2", "c2", "e3", "c3", "e4", "f4", "b2", "e5", "c4", "e2", "c1")

	assertMoves(t, rightMoves,
		"h8", "h6", "g7", "i6", "g8", "i7", "j6", "f8", "i8", "g9", "i5", "g6")
}

func TestFindKnightMoves(t *testing.T) {
	game1 := EmptyGame()
	game1.SetPiece("f6", WhiteKnight)
	game2 := EmptyGame()
	game2.SetPiece("d3", WhiteKnight)
	game3 := EmptyGame()
	game3.SetPiece("h7", WhiteKnight)

	centerMoves := game1.FindKnightMoves(Hexagon{File: 5, Rank: 5}).Moves
	leftMoves := game2.FindKnightMoves(Hexagon{File: 3, Rank: 2}).Moves
	rightMoves := game3.FindKnightMoves(Hexagon{File: 7, Rank: 6}).Moves

	assertMoves(t, centerMoves,
		"h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4")

	assertMoves(t, leftMoves,
		"f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3")

	assertMoves(t, rightMoves,
		"j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5")
}

func TestFindPawnMoves(t *testing.T) {
	game1 := EmptyGame()
	game1.SetPiece("g4", WhitePawn)
	game2 := EmptyGame()
	game2.
		SetPiece("c4", BlackKnight).
		SetPiece("e5", WhiteKnight).
		SetPiece("d5", BlackPawn)

	firstMoves := game1.FindPawnMoves(Hexagon{File: 6, Rank: 3}, true).Moves
	takeMoves := game2.FindPawnMoves(Hexagon{File: 3, Rank: 4}, false).Moves

	assertMoves(t, firstMoves, "g5", "g6")
	assertMoves(t, takeMoves, "d4", "e5")
}
