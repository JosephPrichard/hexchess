package chess

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
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
	game1.SetPieces(
		Move{"f6", WhiteKing},
		Move{"f4", BlackQueen},
		Move{"f8", BlackQueen},
		Move{"b4", BlackBishop},
		Move{"j4", BlackBishop},
		Move{"f9", BlackKing})

	game2 := MakeEmptyGame()
	game2.SetPieces(
		Move{"f1", WhiteKing},
		Move{"a1", BlackQueen},
		Move{"h1", BlackRook},
		Move{"f3", BlackRook},
		Move{"f9", BlackKing})

	for i, game := range []Game{game1, game2} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			game.InitPieceMoves()
			t.Logf("game:\n%s", game.StringColor(!game.Board.IsWhiteTurn))
			isCheckmate := game.Checkmate()
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

func TestGame_findMoves(t *testing.T) {
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
			f:        (*Game).findRookMoves,
			expMoves: []string{"f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1", "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6"},
		},
		{
			name:     "RookLeft",
			game:     MakeStartGame(Move{"c8", BlackRook}),
			hex:      Hex{File: 2, Rank: 7},
			f:        (*Game).findRookMoves,
			expMoves: []string{"d8", "e8", "f8"},
		},
		{
			name:     "RookRight",
			game:     MakeStartGame(Move{"h4", BlackRook}),
			hex:      Hex{File: 7, Rank: 3},
			f:        (*Game).findRookMoves,
			expMoves: []string{"h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6", "d6", "c6", "b6", "a6", "i4", "j4", "k4"},
		},
		{
			name:     "BishopCenter",
			game:     MakeStartGame(Move{"f6", BlackBishop}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).findBishopMoves,
			expMoves: []string{"h5", "j4", "d5", "b4", "g4", "e4"},
		},
		{
			name:     "BishopLeft",
			game:     MakeStartGame(Move{"c8", BlackBishop}),
			hex:      Hex{File: 2, Rank: 7},
			f:        (*Game).findBishopMoves,
			expMoves: []string{"e9", "g9", "b6", "a4"},
		},
		{
			name:     "BishopRight",
			game:     MakeStartGame(Move{"h4", BlackBishop}),
			hex:      Hex{File: 7, Rank: 3},
			f:        (*Game).findBishopMoves,
			expMoves: []string{"j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2"},
		},
		{
			name:     "KnightCenter",
			game:     MakeEmptyGame(Move{"f6", WhiteKnight}),
			hex:      Hex{File: 5, Rank: 5},
			f:        (*Game).findKnightMoves,
			expMoves: []string{"h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4"},
		},
		{
			name:     "KnightLeft",
			game:     MakeEmptyGame(Move{"d3", WhiteKnight}),
			hex:      Hex{File: 3, Rank: 2},
			f:        (*Game).findKnightMoves,
			expMoves: []string{"f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3"},
		},
		{
			name:     "KnightRight",
			game:     MakeEmptyGame(Move{"h7", WhiteKnight}),
			hex:      Hex{File: 7, Rank: 6},
			f:        (*Game).findKnightMoves,
			expMoves: []string{"j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5"},
		},
		{
			name:     "PawnFirstMove",
			game:     MakeStartGame(Move{"g4", WhitePawn}),
			hex:      Hex{File: 6, Rank: 3},
			f:        (*Game).findPawnMovesWhite,
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
			f:        (*Game).findPawnMovesBlack,
			expMoves: []string{"d4", "e5"},
		},
	} {
		t.Run(fmt.Sprintf("%s", test.name), func(t *testing.T) {
			t.Logf("testing moves for piece at: %v on game:%s", test.hex, test.game.Board.String())
			moves := test.f(&test.game, test.hex).Moves
			t.Logf("after move: %v on game:%s", test.hex, test.game.Board.StringMoves(moves))
			assertMoves(t, moves, test.expMoves...)
		})
	}
}

func TestGame_MakeMove(t *testing.T) {
	game := MakeStartGame(
		Move{"c4", BlackKnight},
		Move{"e5", WhiteKnight},
		Move{"d5", BlackPawn})
	game.MakeMove(Hex{File: 3, Rank: 4}, Hex{File: 3, Rank: 3})

	assert.Equal(t, Empty, game.Board.Pieces[3][4])
	assert.Equal(t, BlackPawn, game.Board.Pieces[3][3])
	assert.Equal(t, 1, len(game.MoveList))
}

type PieceMoveNode struct {
	Move PieceMove
	Game Game
	Prev *PieceMoveNode
}

type BadGame struct {
	Game  Game
	Moves []Hex
	Node  *PieceMoveNode
}

func findBadGames(game Game, depth int, node *PieceMoveNode, b *BadGame) {
	if depth == 0 {
		return
	}
	game.InitPieceMoves()
	if game.Checkmate() {
		return
	}
	for _, pms := range game.GetCurrMoves() {
		for _, to := range pms.Moves {
			toPiece := game.Board.Pieces[to.File][to.Rank]
			if game.Board.IsWhiteTurn && toPiece == BlackKing || !game.Board.IsWhiteTurn && toPiece == WhiteKing {
				*b = BadGame{Game: game, Node: node, Moves: pms.Moves}
				panic("bad game")
				return
			}
			game2 := game.MakeMoved(pms.From, to)
			nextNode := &PieceMoveNode{
				Game: game,
				Move: PieceMove{
					Piece: pms.Piece,
					From:  pms.From,
					To:    to,
				},
				Prev: node,
			}
			findBadGames(game2, depth-1, nextNode, b)
		}
	}
}

func TestGame_NoKingCheckMove(t *testing.T) {
	var b BadGame
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("bad game: %s\n", b.Game.Board.StringMoves(b.Moves))
			node := b.Node
			for node != nil {
				fmt.Printf("bad move: %s node: %s\n", node.Move, node.Game.Board.String())
				node = node.Prev
			}
			t.FailNow()
		}
	}()
	findBadGames(MakeStartGame(), 5, nil, &b)
}
