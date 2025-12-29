package chess

import (
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
	assert.Equal(t, Hex{File: 5, Rank: 8}, HexStr("f9"))
	assert.Equal(t, Hex{File: 6, Rank: 9}, HexStr("g10"))
	assert.Equal(t, Hex{File: 4, Rank: 4}, HexStr("e5"))
}

func TestGame_GetSetPieces(t *testing.T) {
	board := InitialBoard()
	board.SetPieceNot("f3", WhiteBishop)
	piece := board.GetPieceNot("f3")
	assert.Equal(t, WhiteBishop, piece)
}

func TestGame_DetermineIsCheckmate(t *testing.T) {
	game1 := MakeEmptyGame(true)
	game1.SetPieces(
		NotMove{"f6", WhiteKing},
		NotMove{"f4", BlackQueen},
		NotMove{"f8", BlackQueen},
		NotMove{"b4", BlackBishop},
		NotMove{"j4", BlackBishop},
		NotMove{"f9", BlackKing})

	game2 := MakeEmptyGame(true)
	game2.SetPieces(
		NotMove{"f1", WhiteKing},
		NotMove{"a1", BlackQueen},
		NotMove{"h1", BlackRook},
		NotMove{"f3", BlackRook},
		NotMove{"f9", BlackKing})

	for _, test := range []struct {
		name string
		game Game
	}{
		{name: "surronded king is checked", game: game1},
		{name: "cornered king is checked", game: game2},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &test.game
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
		expectedMoves = append(expectedMoves, HexStr(s))
	}
	assert.ElementsMatch(t, expectedMoves, actual)
}

func TestGame_findMoves(t *testing.T) {
	for _, test := range []struct {
		name      string
		game      Game
		hex       Hex
		fn        func(*Game, Hex) PieceMoves
		wantMoves []string
	}{
		{
			name:      "center rook",
			game:      MakeStartGame(NotMove{"f6", BlackRook}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1", "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6"},
		},
		{
			name:      "rook left",
			game:      MakeStartGame(NotMove{"c8", BlackRook}),
			hex:       Hex{File: 2, Rank: 7},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"d8", "e8", "f8"},
		},
		{
			name:      "rook right",
			game:      MakeStartGame(NotMove{"h4", BlackRook}),
			hex:       Hex{File: 7, Rank: 3},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6", "d6", "c6", "b6", "a6", "i4", "j4", "k4"},
		},
		{
			name:      "bishop center",
			game:      MakeStartGame(NotMove{"f6", BlackBishop}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"h5", "j4", "d5", "b4", "g4", "e4"},
		},
		{
			name:      "bishop left",
			game:      MakeStartGame(NotMove{"c8", BlackBishop}),
			hex:       Hex{File: 2, Rank: 7},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"e9", "g9", "b6", "a4"},
		},
		{
			name:      "bishop right",
			game:      MakeStartGame(NotMove{"h4", BlackBishop}),
			hex:       Hex{File: 7, Rank: 3},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2"},
		},
		{
			name:      "knight center",
			game:      MakeEmptyGame(true, NotMove{"f6", WhiteKnight}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4"},
		},
		{
			name:      "knight left",
			game:      MakeEmptyGame(true, NotMove{"d3", WhiteKnight}),
			hex:       Hex{File: 3, Rank: 2},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3"},
		},
		{
			name:      "knight right",
			game:      MakeEmptyGame(true, NotMove{"h7", WhiteKnight}),
			hex:       Hex{File: 7, Rank: 6},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5"},
		},
		{
			name:      "pawn first move",
			game:      MakeStartGame(NotMove{"g4", WhitePawn}),
			hex:       Hex{File: 6, Rank: 3},
			fn:        (*Game).findPawnMovesWhite,
			wantMoves: []string{"g5", "g6"},
		},
		{
			name: "pawn take move",
			game: MakeStartGame(
				NotMove{"c4", BlackKnight},
				NotMove{"e5", WhiteKnight},
				NotMove{"d5", BlackPawn},
			),
			hex:       Hex{File: 3, Rank: 4},
			fn:        (*Game).findPawnMovesBlack,
			wantMoves: []string{"d4", "e5"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("testing moves for piece at: %v on game:%s", test.hex, test.game.Board.String())
			moves := test.fn(&test.game, test.hex).Moves
			t.Logf("after move: %v on game:%s", test.hex, test.game.Board.StringMoves(moves))
			assertMoves(t, moves, test.wantMoves...)
		})
	}
}

func TestGame_MakeMove(t *testing.T) {
	game := MakeStartGame(
		NotMove{"c4", BlackKnight},
		NotMove{"e5", WhiteKnight},
		NotMove{"d5", WhitePawn},
		NotMove{"k5", WhitePawn})
	game1 := game.MakeMoved(Move{From: Hex{File: 3, Rank: 4}, To: Hex{File: 3, Rank: 3}, Promotion: QueenPromotion})

	assert.Equal(t, Empty, game1.Board.Get(3, 4))
	assert.Equal(t, WhitePawn, game1.Board.Get(3, 3))
	assert.Equal(t, 1, len(game1.Moves))

	game2 := game.MakeMoved(Move{From: Hex{File: 10, Rank: 4}, To: Hex{File: 10, Rank: 5}, Promotion: QueenPromotion})

	assert.Equal(t, Empty, game2.Board.Get(10, 4))
	assert.Equal(t, WhiteQueen, game2.Board.Get(10, 5))
	assert.Equal(t, 1, len(game2.Moves))
}

func TestGame_GetMoveNotation(t *testing.T) {
	game := MakeEmptyGame(true,
		NotMove{"f5", WhitePawn},
		NotMove{"a1", WhiteKing},
		NotMove{"k6", BlackKing},
		NotMove{"g8", WhiteRook},
	)
	game.InitPieceMoves()

	for _, test := range []struct {
		game Game
		hm   HistMove
		not  string
	}{
		{
			game: game,
			hm: HistMove{
				PieceMove: PieceMove{
					Piece: WhitePawn,
					From:  HexStr("e5"),
					To:    HexStr("f6"),
				},
			},
			not: "Pef6",
		},
		{
			game: game,
			hm: HistMove{
				PieceMove: PieceMove{
					Piece: WhitePawn,
					From:  HexStr("f7"),
					To:    HexStr("f6"),
				},
			},
			not: "P7f6",
		},
		{
			game: game,
			hm: HistMove{
				PieceMove: PieceMove{
					Piece: WhitePawn,
					From:  HexStr("k1"),
					To:    HexStr("f6"),
				},
			},
			not: "Pf6",
		},
		{
			game: game,
			hm: HistMove{
				PieceMove: PieceMove{
					Piece: WhitePawn,
					From:  HexStr("e5"),
					To:    HexStr("f5"),
				},
				Promotion: QueenPromotion,
			},
			not: "Pxf5=Q",
		},
		//{
		//	game: game,
		//	hm: HistMove{
		//		PieceMove: PieceMove{
		//			Piece: WhiteRook,
		//			From:  HexStr("g9"),
		//			To:    HexStr("g8"),
		//		},
		//	},
		//	not: "R+g8",
		//},
	} {
		t.Logf("testing get move notation on game:%s", test.game.Board.StringMoves(test.game.GetCurrMoves()[0].Moves))
		t.Logf("expecting move: %v", test.not)

		histMove := test.game.AnnotateHistMove(test.hm)
		str := histMove.String()
		assert.Equal(t, test.not, str)
	}
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
			game2 := game.MakeMoved(Move{From: pms.From, To: to, Promotion: QueenPromotion})
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

//func TestGame_NoKingCheckMove(t *testing.T) {
//	var b BadGame
//	defer func() {
//		if r := recover(); r != nil {
//			fmt.Printf("bad game: %s\n", b.Game.Board.StringMoves(b.Moves))
//			node := b.Node
//			for node != nil {
//				fmt.Printf("bad move: %s node: %s\n", node.Move, node.Game.Board.String())
//				node = node.Prev
//			}
//			t.FailNow()
//		}
//	}()
//	findBadGames(MakeStartGame(), 5, nil, &b)
//}
