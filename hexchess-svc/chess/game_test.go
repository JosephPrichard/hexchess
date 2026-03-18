package chess

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Place{"f6", WhiteKing},
		Place{"f4", BlackQueen},
		Place{"f8", BlackQueen},
		Place{"b4", BlackBishop},
		Place{"j4", BlackBishop},
		Place{"f9", BlackKing})

	game2 := MakeEmptyGame(true)
	game2.SetPieces(
		Place{"f1", WhiteKing},
		Place{"a1", BlackQueen},
		Place{"h1", BlackRook},
		Place{"f3", BlackRook},
		Place{"f9", BlackKing})

	for _, test := range []struct {
		name string
		game Game
	}{
		{name: "surronded king is checkmated", game: game1},
		{name: "cornered king is checkmated", game: game2},
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
	assert.Equal(t, expectedMoves, actual)
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
			game:      MakeStartGame(Place{"f6", BlackRook}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"f5", "e5", "d4", "c3", "b2", "a1", "g5", "h4", "i3", "j2", "k1", "e6", "d6", "c6", "b6", "a6", "g6", "h6", "i6", "j6", "k6"},
		},
		{
			name:      "rook left",
			game:      MakeStartGame(Place{"c8", BlackRook}),
			hex:       Hex{File: 2, Rank: 7},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"d8", "e8", "f8"},
		},
		{
			name:      "rook right",
			game:      MakeStartGame(Place{"h4", BlackRook}),
			hex:       Hex{File: 7, Rank: 3},
			fn:        (*Game).findRookMoves,
			wantMoves: []string{"h5", "h6", "h3", "g4", "i3", "j2", "k1", "g5", "f6", "e6", "d6", "c6", "b6", "a6", "i4", "j4", "k4"},
		},
		{
			name:      "bishop center",
			game:      MakeStartGame(Place{"f6", BlackBishop}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"h5", "j4", "d5", "b4", "g4", "e4"},
		},
		{
			name:      "bishop left",
			game:      MakeStartGame(Place{"c8", BlackBishop}),
			hex:       Hex{File: 2, Rank: 7},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"e9", "g9", "b6", "a4"},
		},
		{
			name:      "bishop right",
			game:      MakeStartGame(Place{"h4", BlackBishop}),
			hex:       Hex{File: 7, Rank: 3},
			fn:        (*Game).findBishopMoves,
			wantMoves: []string{"j3", "f5", "i5", "j6", "g6", "f8", "e9", "i2", "g3", "f2"},
		},
		{
			name:      "knight center",
			game:      MakeEmptyGame(true, Place{"f6", WhiteKnight}),
			hex:       Hex{File: 5, Rank: 5},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"h7", "g8", "h3", "g3", "d7", "e8", "d3", "e3", "c5", "c4", "i5", "i4"},
		},
		{
			name:      "knight left",
			game:      MakeEmptyGame(true, Place{"d3", WhiteKnight}),
			hex:       Hex{File: 3, Rank: 2},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"f6", "e6", "f2", "e1", "b4", "c5", "a2", "a1", "g4", "g3"},
		},
		{
			name:      "knight right",
			game:      MakeEmptyGame(true, Place{"h7", WhiteKnight}),
			hex:       Hex{File: 7, Rank: 6},
			fn:        (*Game).findKnightMoves,
			wantMoves: []string{"j4", "i4", "f10", "g10", "f6", "g5", "e8", "e7", "k6", "k5"},
		},
		{
			name:      "pawn first move",
			game:      MakeStartGame(Place{"g4", WhitePawn}),
			hex:       Hex{File: 6, Rank: 3},
			fn:        (*Game).findPawnMovesWhite,
			wantMoves: []string{"g5", "g6"},
		},
		{
			name: "pawn take move",
			game: MakeStartGame(
				Place{"c4", BlackKnight},
				Place{"e5", WhiteKnight},
				Place{"d5", BlackPawn},
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

func TestGame_ValidateMove(t *testing.T) {
	// given
	game := MakeStartGame(
		Place{"a1", WhiteKnight},
		Place{"a5", WhitePawn},
		Place{"c4", BlackKnight},
		Place{"e5", WhiteKnight},
		Place{"d5", WhitePawn},
		Place{"k5", WhitePawn})
	game.InitPieceMoves()

	t.Logf("game:\n%s", game.Board.String())

	for _, test := range []struct {
		name     string
		move     Move
		wantKind MoveErrorKind
	}{
		{
			name:     "valid move",
			move:     MoveStr("d5", "d6"),
			wantKind: -1,
		},
		{
			name:     "noop move",
			move:     MoveStr("d5", "d5"),
			wantKind: MoveErrNoop,
		},
		{
			name:     "ext of bounds move",
			move:     Move{From: Hex{File: 1, Rank: 1}, To: Hex{File: 14, Rank: 1}},
			wantKind: MoveErrOutOfBounds,
		},
		{
			name:     "invalid promotion (invalid enum)",
			move:     Move{From: HexStr("a5"), To: HexStr("a6"), Promotion: 1000},
			wantKind: MoveErrInvalidPromotion,
		},
		{
			name:     "invalid promotion (zero value)",
			move:     MoveStr("a5", "a6"),
			wantKind: MoveErrInvalidPromotion,
		},
		{
			name:     "cannot move to",
			move:     MoveStr("a1", "a2"),
			wantKind: MoveErrIllegalDestination,
		},
		{
			name:     "cannot move from",
			move:     MoveStr("a2", "a3"),
			wantKind: MoveErrIllegalTarget,
		},
	} {
		t.Run(test.name, func(t *testing.T) { // when
			err := game.ValidateMove(test.move)

			// then
			if test.wantKind >= 0 {
				assert.Equal(t, MoveError{Kind: test.wantKind, Move: test.move}, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGame_MakeMove(t *testing.T) {
	game := MakeStartGame(
		Place{"c4", BlackKnight},
		Place{"e5", WhiteKnight},
		Place{"d5", WhitePawn},
		Place{"k5", WhitePawn})
	game1 := game.MakeMoved(Move{From: Hex{File: 3, Rank: 4}, To: Hex{File: 3, Rank: 3}})

	assert.Equal(t, Empty, game1.Board.Get(3, 4))
	assert.Equal(t, WhitePawn, game1.Board.Get(3, 3))
	game2 := game.MakeMoved(Move{From: Hex{File: 10, Rank: 4}, To: Hex{File: 10, Rank: 5}, Promotion: QueenPromotion})

	assert.Equal(t, Empty, game2.Board.Get(10, 4))
	assert.Equal(t, WhiteQueen, game2.Board.Get(10, 5))
}

func TestGame_GetMoveNotation(t *testing.T) {
	game := MakeEmptyGame(true,
		Place{"f5", WhitePawn},
		Place{"a1", WhiteKing},
		Place{"k6", BlackKing},
		Place{"g8", WhiteRook},
	)
	game.InitPieceMoves()

	for _, test := range []struct {
		name      string
		game      Game
		pm        PieceMove
		promotion Promotion
		not       string
	}{
		{
			name: "pawn move colision file",
			game: game,
			pm: PieceMove{
				Piece: WhitePawn,
				From:  HexStr("e5"),
				To:    HexStr("f6"),
			},
			not: "Pef6",
		},
		{
			name: "pawn move collision rank",
			game: game,
			pm: PieceMove{
				Piece: WhitePawn,
				From:  HexStr("f7"),
				To:    HexStr("f6"),
			},
			not: "P7f6",
		},
		{
			name: "pawn move no collision",
			game: game,
			pm: PieceMove{
				Piece: WhitePawn,
				From:  HexStr("k1"),
				To:    HexStr("f6"),
			},
			not: "Pf6",
		},
		{
			name: "pawn promotion",
			game: game,
			pm: PieceMove{
				Piece: WhitePawn,
				From:  HexStr("e5"),
				To:    HexStr("f5"),
			},
			promotion: QueenPromotion,
			not:       "Pf5=Q",
		},
		{
			name: "moving white king",
			game: game,
			pm: PieceMove{
				Piece: WhiteKing,
				From:  HexStr("a1"),
				To:    HexStr("a2"),
			},
			not: "Ka2",
		},
		{
			name: "moving black king",
			game: game,
			pm: PieceMove{
				Piece: BlackKing,
				From:  HexStr("k6"),
				To:    HexStr("k7"),
			},
			not: "kk7",
		},
		//{
		//	name: "check",
		//	game: game,
		//	pm: PieceMove{
		//		Piece: WhiteRook,
		//		From:  HexStr("g9"),
		//		To:    HexStr("g8"),
		//	},
		//	not: "R+g8",
		//},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("testing get move notation on game:%s", test.game.Board.StringMoves(test.game.GetCurrMoves()[0].Moves))
			t.Logf("expecting move: %v", test.not)

			test.game.InitPieceMoves()
			annotMove := test.game.MakeAnnotatedMove(test.pm)
			annotMove.Promotion = test.promotion
			str := annotMove.String()
			assert.Equal(t, test.not, str)
		})
	}
}

func TestJumpMoveIndex(t *testing.T) {
	t.Run("no move to jump to", func(t *testing.T) {
		var moves []HistMove

		game, err := JumpMoveIndex(MakeStartBoard(), moves, 1)

		require.Equal(t, JumpIndexError{Count: 2, Len: 0}, err)
		require.Nil(t, game)
	})

	t.Run("jumping to first move", func(t *testing.T) {
		moves := ApplyMoveSeq(
			Move{From: HexStr("b1"), To: HexStr("b2")},
			Move{From: HexStr("b7"), To: HexStr("b6")})

		game, err := JumpMoveIndex(MakeStartBoard(), moves, 0)

		require.NoError(t, err)
		require.NotNil(t, game)
		assert.Len(t, game.Moves, 1)
	})

	t.Run("jumping to last move (noop)", func(t *testing.T) {
		moves := ApplyMoveSeq(
			Move{From: HexStr("b1"), To: HexStr("b2")},
			Move{From: HexStr("b7"), To: HexStr("b6")})

		game, err := JumpMoveIndex(MakeStartBoard(), moves, 1)

		require.NoError(t, err)
		require.NotNil(t, game)
		assert.Len(t, game.Moves, 2)
	})

	t.Run("jumping to second move", func(t *testing.T) {
		moves := ApplyMoveSeq(
			Move{From: HexStr("b1"), To: HexStr("b2")},
			Move{From: HexStr("b7"), To: HexStr("b6")},
			Move{From: HexStr("c2"), To: HexStr("c3")})

		game, err := JumpMoveIndex(MakeStartBoard(), moves, 1)

		require.NoError(t, err)
		require.NotNil(t, game)
		assert.Len(t, game.Moves, 2)
	})
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

//func TestGame_NoKingCheckMove(t(t *testing.T) {
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
