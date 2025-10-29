package chess

import (
	"errors"
	"fmt"
	"math/rand"
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

func MakeStartGame(initial ...Move) Game {
	game := Game{Board: InitialBoard()}
	for _, pm := range initial {
		game.SetPiece(pm.Not, pm.Piece)
	}
	return game
}

func MakeEmptyGame(initial ...Move) Game {
	game := Game{Board: MakeBoard(true)}
	for _, pm := range initial {
		game.SetPiece(pm.Not, pm.Piece)
	}
	return game
}

func (g *Game) DeepCopy() Game {
	g2 := Game{}

	g2.Board = g.Board
	g2.WhiteMoves = append(g2.WhiteMoves, g.WhiteMoves...)
	g2.BlackMoves = append(g2.BlackMoves, g.BlackMoves...)
	g2.TakenWhitePieces = append(g2.TakenWhitePieces, g.TakenWhitePieces...)
	g2.TakenBlackPieces = append(g2.TakenBlackPieces, g.TakenBlackPieces...)

	return g2
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

func (g *Game) ClearCurrentMoves() {
	if g.Board.IsWhiteTurn {
		g.WhiteMoves = g.WhiteMoves[:0]
	} else {
		g.BlackMoves = g.BlackMoves[:0]
	}
}

func (g *Game) GetOppositeMoves() []PieceMoves {
	return g.GetTurnMoves(!g.Board.IsWhiteTurn)
}

func (g *Game) MakeMoved(from, to Hex) Game {
	g2 := g.DeepCopy()
	g2.MakeMove(from, to)
	return g2
}

func (g *Game) MakeMove(from, to Hex) PieceMove {
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
	if len(g.WhiteMoves) == 0 || len(g.BlackMoves) == 0 {
		g.InitPieceMoves()
	}
	moves := g.GetCurrMoves()

	for _, pm := range moves {
		isFrom := pm.From == move.From
		hasTo := slices.ContainsFunc(pm.Moves, func(to Hex) bool { return to == move.To })
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
	kingHex := blackKingHex
	if g.Board.IsWhiteTurn {
		kingHex = whiteKingHex
	}

	// to decide whether we will add the piece moves, are we in check?
	// we don't need to check if the opposite king is in check, it should never be!
	isAttacked := g.FindAttackTable()
	isCheck := isAttacked[kingHex.File][kingHex.Rank]

	if isCheck {
		// if the current king is in check, we cannot move any other pieces
		// TODO: add support for maintaining all "blocking" moves
		g.ClearCurrentMoves()
	}

	// find the moves for both kings - excluding any attacking squares
	canMoveTo := func(hex Hex) bool {
		return !isAttacked[kingHex.File][kingHex.Rank]
	}
	g.WhiteMoves = append(g.WhiteMoves, g.FindKingMovesFiltered(whiteKingHex, canMoveTo))
	g.BlackMoves = append(g.BlackMoves, g.FindKingMovesFiltered(blackKingHex, canMoveTo))
}

func (g *Game) FindAttackTable() [Files][MaxRanks]bool {
	moves := g.GetOppositeMoves()

	var isAttacked [Files][MaxRanks]bool

	for _, pm := range moves {
		piece := g.Board.Pieces[pm.From.File][pm.From.Rank]
		for _, move := range pm.Moves {
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
	//kingHex := g.Board.FindKing(g.Board.IsWhiteTurn)
	//
	//isAttacked := g.FindAttackTable()
	//isChecked := isAttacked[kingHex.File][kingHex.Rank]
	//if !isChecked {
	//	return false
	//}
	//
	//
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

func (g *Game) FindRookMoves(hex Hex) PieceMoves {
	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: g.FindMovesByTraveling(hex, RookOffsets)}
}

func (g *Game) FindBishopMoves(hex Hex) PieceMoves {
	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: g.FindMovesByTraveling(hex, BishopOffsets)}
}

func (g *Game) FindQueenMoves(hex Hex) PieceMoves {
	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: g.FindMovesByTraveling(hex, KingOffsets)}
}

func (g *Game) FindKnightMoves(hex Hex) PieceMoves {
	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: g.FindOffsetMoves(hex, KnightOffsets)}
}

func (g *Game) FindKingMoves(hex Hex) PieceMoves {
	// we want all moves, so nothing is attacking
	return g.FindKingMovesFiltered(hex, func(h Hex) bool { return true })
}

func (g *Game) FindKingMovesFiltered(hex Hex, canMoveTo func(Hex) bool) PieceMoves {
	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: g.FindOffsetMovesFiltered(hex, KingOffsets, canMoveTo)}
}

func (g *Game) FindPawnMovesWhite(hex Hex) PieceMoves {
	return g.FindPawnMoves(hex, true)
}

func (g *Game) FindPawnMovesBlack(hex Hex) PieceMoves {
	return g.FindPawnMoves(hex, false)
}

func (g *Game) FindPawnMoves(hex Hex, isWhiteTurn bool) PieceMoves {
	pick := func(cond bool, a, b []Direction) []Direction {
		if cond {
			return a
		}
		return b
	}

	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hex

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

	return PieceMoves{Piece: g.Board.Pieces[hex.File][hex.Rank], From: hex, Moves: moves}
}

func (g *Game) FindMovesByTraveling(hex Hex, directions [][]Direction) []Hex {
	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hex

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

func (g *Game) FindOffsetMoves(hex Hex, directions [][]Direction) []Hex {
	return g.FindOffsetMovesFiltered(hex, directions, func(Hex) bool { return true })
}

func (g *Game) FindOffsetMovesFiltered(hex Hex, directions [][]Direction, canMoveTo func(Hex) bool) []Hex {
	basePiece := g.Board.Pieces[hex.File][hex.Rank]
	var moves []Hex

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

func (g *Game) String() string {
	return g.StringColor(g.Board.IsWhiteTurn)
}

func (g *Game) StringColor(isWhite bool) string {
	if len(g.WhiteMoves) == 0 || len(g.BlackMoves) == 0 {
		g.InitPieceMoves()
	}

	moves := g.WhiteMoves
	kingMoves := g.BlackMoves[len(g.BlackMoves)-1]
	if !isWhite {
		moves = g.BlackMoves
		kingMoves = g.WhiteMoves[len(g.WhiteMoves)-1]
	}

	var moveTable [Files][MaxRanks]rune

	for _, move := range kingMoves.Moves {
		moveTable[move.File][move.Rank] = '_'
	}
	for _, pm := range moves {
		for _, move := range pm.Moves {
			moveTable[move.File][move.Rank] = 'x'
		}
	}

	return g.Board.StringFunc(func(hex Hex) rune {
		return moveTable[hex.File][hex.Rank]
	})
}

func RandomMoveList(game Game, low int, hi int) ([]PieceMove, error) {
	var moveList []PieceMove

	// generates a random move list for mock data between length 35 and 45
	for range rand.Intn(low) + (low + hi) {
		game.InitPieceMoves()

		var pmsList []PieceMoves
		for _, pm := range game.GetCurrMoves() {
			if len(pm.Moves) > 0 {
				pmsList = append(pmsList, pm)
			}
		}
		if len(pmsList) == 0 {
			return nil, errors.New("reached checkmate or stalemate")
		}

		pms := pmsList[rand.Intn(len(pmsList))]
		if len(pms.Moves) == 0 {
			return nil, fmt.Errorf("expected at least one move, got none for game: %v", game)
		}
		pm := PieceMove{
			Piece: game.Board.Pieces[pms.From.File][pms.From.Rank],
			From:  pms.From,
			To:    pms.Moves[rand.Intn(len(pms.Moves))],
		}

		if game.Board.Pieces[pm.To.File][pm.To.Rank].IsKing() {
			panic("assertion error: should never be allowed to make a move to the king")
		}

		game.MakeMove(pm.From, pm.To)
		moveList = append(moveList, pm)
	}

	return moveList, nil
}
