package chess

import (
	"fmt"
	"slices"
)

type Game struct {
	Board            Board
	WhiteMoves       []PieceMoves
	BlackMoves       []PieceMoves
	TakenWhitePieces []Piece
	TakenBlackPieces []Piece
}

var (
	WhiteAhead     = []Direction{Up}
	WhiteTakeLeft  = []Direction{UpLeft}
	WhiteTakeRight = []Direction{UpRight}
	BlackAhead     = []Direction{Down}
	BlackTakeLeft  = []Direction{DownLeft}
	BlackTakeRight = []Direction{DownRight}
)

type Move struct {
	Not   string
	Piece Piece
}

func StartGame(initial ...Move) Game {
	game := Game{Board: InitialBoard()}
	for _, pm := range initial {
		game.SetPiece(pm.Not, pm.Piece)
	}
	return game
}

func EmptyGame(initial ...Move) Game {
	game := Game{Board: MakeBoard(false)}
	for _, pm := range initial {
		game.SetPiece(pm.Not, pm.Piece)
	}
	return game
}

func (g *Game) SetPiece(str string, p Piece) *Game {
	hex := ParseHexagonValid(str)
	g.Board.Pieces[hex.File][hex.Rank] = p
	return g
}

func (g *Game) GetTurnMoves(isWhiteTurn bool) []PieceMoves {
	if isWhiteTurn {
		return g.WhiteMoves
	}
	return g.BlackMoves
}

func (g *Game) GetCurrMoves() []PieceMoves {
	return g.GetTurnMoves(g.Board.IsWhiteTurn)
}

func (g *Game) GetOppositeMoves() []PieceMoves {
	return g.GetTurnMoves(!g.Board.IsWhiteTurn)
}

func (g *Game) MakeMove(from, to Hexagon) PieceMove {
	piece1 := g.Board.Pieces[from.File][from.Rank]
	piece2 := g.Board.Pieces[to.File][to.Rank]

	if piece2 != Empty {
		if g.Board.IsWhiteTurn {
			g.TakenWhitePieces = append(g.TakenWhitePieces, piece2)
		} else {
			g.TakenBlackPieces = append(g.TakenBlackPieces, piece2)
		}
	}

	g.Board.Pieces[from.File][from.Rank] = Empty
	g.Board.Pieces[to.File][to.Rank] = piece1
	g.Board.IsWhiteTurn = !g.Board.IsWhiteTurn

	return PieceMove{Piece: piece1, From: from, To: to}
}

func (g *Game) IsValidMove(move PieceMove) bool {
	moves := g.GetCurrMoves()

	for _, pm := range moves {
		isFrom := pm.From == move.From
		hasTo := slices.ContainsFunc(pm.Moves, func(to Hexagon) bool { return to == move.To })
		if isFrom && hasTo {
			return true
		}
	}

	return false
}

func (g *Game) InitPieceMoves() {
	// we find the piece moves for all other pieces besides the king
	g.WhiteMoves = g.FindPieceMoves(true)
	g.BlackMoves = g.FindPieceMoves(false)

	whiteKingHex := g.Board.FindKing(true)
	blackKingHex := g.Board.FindKing(false)

	// find the moves for both kings - excluding any attacking squares
	whiteKingMoves := g.FindKingMoves(whiteKingHex)
	blackKingMoves := g.FindKingMoves(blackKingHex)
	kingHex := blackKingHex
	if g.Board.IsWhiteTurn {
		kingHex = whiteKingHex
	}

	// decide whether we will add the piece moves... are we in check?
	// we don't need to check if the opposite move is in check... it should never be!
	currMoves := g.GetCurrMoves()
	oppMoves := g.GetOppositeMoves()
	isAttacked := g.FindAttacking(oppMoves)
	isCheck := isAttacked[kingHex.File][kingHex.Rank]

	if isCheck {
		// if the current king is in check, we cannot move any other pieces
		// TODO: add support for maintaining all "blocking" moves
		currMoves = currMoves[:0] // clear current moves
	}

	// now, we can add the king moves
	g.WhiteMoves = append(g.WhiteMoves, whiteKingMoves)
	g.BlackMoves = append(g.BlackMoves, blackKingMoves)
}

func (g *Game) FindAttacking(moves []PieceMoves) [][]bool {
	isAttacked := make([][]bool, Files)
	for i := 0; i < Files; i++ {
		isAttacked[i] = make([]bool, RanksPerFile[i])
	}

	for _, pm := range moves {
		for _, move := range pm.Moves {
			piece := g.Board.Pieces[pm.From.File][pm.From.Rank]
			isMovingAhead := move.Rank > pm.From.Rank
			if piece.IsPawn() && isMovingAhead {
				continue
			}
			isAttacked[move.File][move.Rank] = true
		}
	}

	return isAttacked
}

func (g *Game) CheckmateReached() bool {
	pickPiece := func(cond bool, a, b Piece) Piece {
		if cond {
			return a
		}
		return b
	}

	isWhiteTurn := g.Board.IsWhiteTurn
	kingHex := g.Board.FindKing(isWhiteTurn)
	pieceMoves := g.GetTurnMoves(isWhiteTurn)
	oppPieceMoves := g.GetTurnMoves(!isWhiteTurn)
	kingMoves := pieceMoves[len(pieceMoves)-1] // last element is always the king moves

	// sanity check: the last piece is the king
	kingPiece := g.Board.Pieces[kingMoves.From.File][kingMoves.From.Rank]
	if kingPiece != pickPiece(isWhiteTurn, WhiteKing, BlackKing) {
		panic("expected last piece to be the king")
	}

	isAttacked := g.FindAttacking(oppPieceMoves)
	isChecked := isAttacked[kingHex.File][kingHex.Rank]
	if !isChecked {
		return false
	}

	// all king moves must be attacked
	for _, move := range kingMoves.Moves {
		if !isAttacked[move.File][move.Rank] {
			return false
		}
	}

	// TODO: support blocking moves and preventing taking defended attackers
	return true
}

func (g *Game) FindPieceMoves(isWhiteTurn bool) []PieceMoves {
	var moves []PieceMoves
	for _, hex := range OrdHexagons {
		piece := g.Board.Pieces[hex.File][hex.Rank]
		if piece == Empty || !piece.IsPieceTurn(isWhiteTurn) {
			continue
		}

		switch piece {
		case WhiteRook, BlackRook:
			moves = append(moves, g.FindRookMoves(hex))
		case WhiteBishop, BlackBishop:
			moves = append(moves, g.FindBishopMoves(hex))
		case WhiteQueen, BlackQueen:
			moves = append(moves, g.FindQueenMoves(hex))
		case WhiteKnight, BlackKnight:
			moves = append(moves, g.FindKnightMoves(hex))
		case WhitePawn, BlackPawn:
			moves = append(moves, g.FindPawnMoves(hex, isWhiteTurn))
		case WhiteKing, BlackKing:
			// no-op; king moves handled elsewhere
		default:
			panic(fmt.Sprintf("board has invalid piece %d at hexagon %v", piece, hex))
		}
	}
	return moves
}

func (g *Game) FindRookMoves(hex Hexagon) PieceMoves {
	return PieceMoves{From: hex, Moves: g.FindMovesByTraveling(hex, RookOffsets)}
}

func (g *Game) FindBishopMoves(hex Hexagon) PieceMoves {
	return PieceMoves{From: hex, Moves: g.FindMovesByTraveling(hex, BishopOffsets)}
}

func (g *Game) FindQueenMoves(hex Hexagon) PieceMoves {
	return PieceMoves{From: hex, Moves: g.FindMovesByTraveling(hex, KingOffsets)}
}

func (g *Game) FindKnightMoves(hex Hexagon) PieceMoves {
	return PieceMoves{From: hex, Moves: g.FindOffsetMoves(hex, KnightOffsets)}
}

func (g *Game) FindKingMoves(hex Hexagon) PieceMoves {
	// we want all moves, so nothing is attacking
	return PieceMoves{From: hex, Moves: g.FindKingMovesFiltered(hex, func(h Hexagon) bool { return true })}
}

func (g *Game) FindKingMovesFiltered(hex Hexagon, isNotAttacked func(Hexagon) bool) []Hexagon {
	return g.FindOffsetMovesFiltered(hex, KingOffsets, isNotAttacked)
}

func (g *Game) FindPawnMovesWhite(hex Hexagon) PieceMoves {
	return g.FindPawnMoves(hex, true)
}

func (g *Game) FindPawnMovesBlack(hex Hexagon) PieceMoves {
	return g.FindPawnMoves(hex, false)
}

func (g *Game) FindPawnMoves(hex Hexagon, isWhiteTurn bool) PieceMoves {
	pick := func(cond bool, a, b []Direction) []Direction {
		if cond {
			return a
		}
		return b
	}

	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hexagon

	move1 := hex.Walk(pick(isWhiteTurn, WhiteAhead, BlackAhead))
	if g.Board.InBoundsHex(move1) && g.Board.Pieces[move1.File][move1.Rank] == Empty {
		moves = append(moves, move1)
	}

	move2 := move1.Walk(pick(isWhiteTurn, WhiteAhead, BlackAhead))
	if g.Board.InBoundsHex(move2) && !HasPawnMoved(hex, basePiece.IsWhite()) && g.Board.Pieces[move2.File][move2.Rank] == Empty {
		moves = append(moves, move2)
	}

	move3 := hex.Walk(pick(isWhiteTurn, WhiteTakeLeft, BlackTakeLeft))
	if g.Board.InBoundsHex(move3) {
		piece := g.Board.Pieces[move3.File][move3.Rank]
		if piece != Empty && AreOpposite(basePiece, piece) {
			moves = append(moves, move3)
		}
	}

	move4 := hex.Walk(pick(isWhiteTurn, WhiteTakeRight, BlackTakeRight))
	if g.Board.InBoundsHex(move4) {
		piece := g.Board.Pieces[move4.File][move4.Rank]
		if piece != Empty && AreOpposite(basePiece, piece) {
			moves = append(moves, move4)
		}
	}

	return PieceMoves{From: hex, Moves: moves}
}

func (g *Game) FindMovesByTraveling(hex Hexagon, directions [][]Direction) []Hexagon {
	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hexagon

	for _, dirSeq := range directions {
		move := hex
		for {
			move = move.Walk(dirSeq)
			if !g.Board.InBoundsHex(move) {
				break
			}

			piece := g.Board.Pieces[move.File][move.Rank]
			if piece == Empty {
				moves = append(moves, move)
			} else {
				if AreOpposite(piece, basePiece) {
					moves = append(moves, move)
				}
				break
			}
		}
	}

	return moves
}

func (g *Game) FindOffsetMoves(hex Hexagon, directions [][]Direction) []Hexagon {
	return g.FindOffsetMovesFiltered(hex, directions, func(Hexagon) bool { return true })
}

func (g *Game) FindOffsetMovesFiltered(hex Hexagon, directions [][]Direction, canMoveTo func(Hexagon) bool) []Hexagon {
	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hexagon

	for _, dirSeq := range directions {
		move := hex.Walk(dirSeq)
		if !g.Board.InBoundsHex(move) {
			continue
		}

		piece := g.Board.Pieces[move.File][move.Rank]
		canMoveHex := piece == Empty || AreOpposite(basePiece, piece)

		if canMoveHex && canMoveTo(move) {
			moves = append(moves, move)
		}
	}

	return moves
}
