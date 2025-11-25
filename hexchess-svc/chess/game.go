package chess

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"slices"
	"strconv"
	"strings"
)

type AttackTable = [Files][MaxRanks]bool

type HistMove struct {
	// stores any information necessary to generate a move history notation
	PieceMove
	CollFile bool
	CollRank bool
	IsCheck  bool
	IsTake   bool
}

func (m *HistMove) String() string {
	from := m.PieceMove.From
	to := m.PieceMove.To

	var sb strings.Builder

	sb.WriteString(m.PieceMove.Piece.String())

	if m.IsCheck {
		sb.WriteString("+")
	}
	if m.IsTake {
		sb.WriteString("x")
	}

	if !m.CollFile || !m.CollRank {
		if m.CollFile {
			sb.WriteString(strconv.Itoa(from.Rank + 1))
		}
		if m.CollRank {
			sb.WriteRune(rune(from.File + 'a'))
		}
	}

	sb.WriteString(to.String())
	return sb.String()
}

type Game struct {
	// data fields that store the state of the game itself
	Board            Board
	WhiteMoves       []PieceMoves
	BlackMoves       []PieceMoves
	TakenWhitePieces []Piece
	TakenBlackPieces []Piece
	Moves            []HistMove
	// keeps track of information while executing InitPieceMoves, zeroed out every time we calculate moves again
	WhiteAttackTable AttackTable
	BlackAttackTable AttackTable
	PinTable         [Files][MaxRanks][]Hex
}

type Move struct {
	Not   string
	Piece Piece
}

func MakeStartGame(initial ...Move) Game {
	return Game{Board: MakeStartBoard(initial...)}
}

func MakeEmptyGame(initial ...Move) Game {
	return Game{Board: MakeEmptyBoard(initial...)}
}

func (g *Game) SetPieces(initial ...Move) {
	for _, move := range initial {
		g.Board.SetPieceNot(move.Not, move.Piece)
	}
}

func (g *Game) DeepCopy() Game {
	game := Game{Board: g.Board}

	for _, pms := range g.WhiteMoves {
		game.WhiteMoves = append(game.WhiteMoves, pms.DeepCopy())
	}
	for _, pms := range g.BlackMoves {
		game.BlackMoves = append(game.BlackMoves, pms.DeepCopy())
	}
	game.Moves = append(game.Moves, g.Moves...)

	game.TakenWhitePieces = append(game.TakenWhitePieces, g.TakenWhitePieces...)
	game.TakenBlackPieces = append(game.TakenBlackPieces, g.TakenBlackPieces...)

	return game
}

func (g *Game) ClearTables() {
	g.BlackAttackTable = [Files][MaxRanks]bool{}
	g.WhiteAttackTable = [Files][MaxRanks]bool{}
	g.PinTable = [Files][MaxRanks][]Hex{}
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

func (g *Game) GetTurnAttackTable(isWhiteTurn bool) *AttackTable {
	if isWhiteTurn {
		return &g.BlackAttackTable
	}
	return &g.WhiteAttackTable
}

func (g *Game) GetCurrAttackTable() *AttackTable {
	return g.GetTurnAttackTable(g.Board.IsWhiteTurn)
}

func (g *Game) clearCurrentMoves() {
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
	game := g.DeepCopy()
	game.MakeMove(from, to)
	return game
}

func (g *Game) MakeMove(from, to Hex) HistMove {
	pieceFrom := g.Board.Get(from.File, from.Rank)
	pieceTo := g.Board.Get(to.File, to.Rank)

	if pieceTo != Empty {
		if g.Board.IsWhiteTurn {
			g.TakenWhitePieces = append(g.TakenWhitePieces, pieceTo)
		} else {
			g.TakenBlackPieces = append(g.TakenBlackPieces, pieceTo)
		}
	}

	move := g.GetHistMove(PieceMove{Piece: pieceFrom, From: from, To: to})

	g.Board.IsWhiteTurn = !g.Board.IsWhiteTurn
	g.Board.Set(from.File, from.Rank, Empty)
	g.Board.Set(to.File, to.Rank, pieceFrom)

	g.Moves = append(g.Moves, move)

	return move
}

type MoveViolation int

const (
	ViolatesNone MoveViolation = iota
	ViolatesOutOfBounds
	ViolatesNoop
	ViolatesIllegalMove
)

func (g *Game) ValidateMove(move PieceMove, checkLegal bool) MoveViolation {
	if move.From.File == move.To.File && move.From.Rank == move.To.Rank {
		return ViolatesNoop
	}
	if !g.Board.InBoundsHex(move.From) || !g.Board.InBoundsHex(move.To) {
		return ViolatesOutOfBounds
	}
	if checkLegal {
		legalMoves := g.GetCurrMoves()
		pmsIdx := slices.IndexFunc(legalMoves, func(moves PieceMoves) bool { return moves.From == move.From })
		if pmsIdx < 0 {
			return ViolatesIllegalMove
		}
		if moveIdx := slices.IndexFunc(legalMoves[pmsIdx].Moves, func(to Hex) bool { return to == move.To }); moveIdx < 0 {
			return ViolatesIllegalMove
		}
	}
	return ViolatesNone
}

func (g *Game) GetHistMove(move PieceMove) HistMove {
	var hm HistMove

	hm.PieceMove = move

	if g.Board.Get(move.To.File, move.To.Rank) != Empty {
		hm.IsTake = true
	}

	moves := g.GetCurrMoves()
	if index := slices.IndexFunc(moves, func(pms PieceMoves) bool { return pms.Piece == move.Piece }); index > 0 {
		for _, h := range moves[index].Moves {
			p := g.Board.Get(h.File, h.Rank)
			if p.IsKing() {
				hm.IsCheck = true
				break
			}
		}
	}

	for _, pms := range moves {
		if pms.Piece != move.Piece {
			continue
		}
		if slices.ContainsFunc(pms.Moves, func(to Hex) bool { return to == move.To }) {
			if pms.From.File == move.From.File {
				hm.CollFile = true
			}
			if pms.From.Rank == move.From.Rank {
				hm.CollRank = true
			}
		}
	}

	return hm
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
	g.ClearTables()

	g.WhiteMoves = g.findPieceMoves(true)
	g.BlackMoves = g.findPieceMoves(false)

	whiteKingHex, wkOk := g.Board.FindKing(true)
	blackKingHex, bkOk := g.Board.FindKing(false)

	handleCheck := func(kingHex Hex) {
		isAttacked := g.GetTurnAttackTable(g.Board.IsWhiteTurn)
		isCheck := isAttacked[kingHex.File][kingHex.Rank]
		if isCheck {
			// TODO: add support for maintaining all "blocking" moves
			g.clearCurrentMoves()
		}
	}
	if g.Board.IsWhiteTurn && wkOk {
		handleCheck(whiteKingHex)
	} else if bkOk {
		handleCheck(blackKingHex)
	}

	if wkOk {
		g.WhiteMoves = append(g.WhiteMoves, g.findKingMovesFiltered(whiteKingHex))
	}
	if bkOk {
		g.BlackMoves = append(g.BlackMoves, g.findKingMovesFiltered(blackKingHex))
	}
}

//func (g *Game) findAttackTable() [Files][MaxRanks]bool {
//	moves := g.GetOppositeMoves()
//
//	var isAttacked [Files][MaxRanks]bool
//
//	for _, pm := range moves {
//		piece := g.Board.Get(pm.From.File, pm.From.Rank)
//		for _, move := range pm.Moves {
//			isMovingAhead := move.Rank > pm.From.Rank
//			if piece.IsPawn() && isMovingAhead {
//				continue
//			}
//			isAttacked[move.File][move.Rank] = true
//		}
//	}
//
//	return isAttacked
//}

func (g *Game) Check() bool {
	kingHex, ok := g.Board.FindKing(g.Board.IsWhiteTurn)
	if !ok {
		return false
	}
	isAttacked := g.GetCurrAttackTable()
	return isAttacked[kingHex.File][kingHex.Rank]
}

func (g *Game) Stalemate() bool {
	count := 0
	for _, pms := range g.GetCurrMoves() {
		count += len(pms.Moves)
	}
	return count == 0
}

func (g *Game) Checkmate() bool {
	return g.Check() && g.Stalemate()
}

func (g *Game) findPieceMoves(isWhiteTurn bool) []PieceMoves {
	var moves []PieceMoves
	for _, hex := range OrdHexagons {
		piece := g.Board.Get(hex.File, hex.Rank)
		if piece == Empty || !piece.IsPieceTurn(isWhiteTurn) {
			continue
		}

		switch piece {
		case WhiteRook, BlackRook:
			moves = append(moves, g.findRookMoves(hex))
		case WhiteBishop, BlackBishop:
			moves = append(moves, g.findBishopMoves(hex))
		case WhiteQueen, BlackQueen:
			moves = append(moves, g.findQueenMoves(hex))
		case WhiteKnight, BlackKnight:
			moves = append(moves, g.findKnightMoves(hex))
		case WhitePawn, BlackPawn:
			moves = append(moves, g.findPawnMoves(hex, isWhiteTurn))
		case WhiteKing, BlackKing:
			// noop; king moves handled elsewhere
		default:
			// noop; ignore invalid pieces
		}
	}
	return moves
}

func (g *Game) findRookMoves(hex Hex) PieceMoves {
	return PieceMoves{
		Piece: g.Board.Get(hex.File, hex.Rank),
		From:  hex,
		Moves: g.findMovesByTraveling(hex, RookOffsets),
	}
}

func (g *Game) findBishopMoves(hex Hex) PieceMoves {
	return PieceMoves{
		Piece: g.Board.Get(hex.File, hex.Rank),
		From:  hex,
		Moves: g.findMovesByTraveling(hex, BishopOffsets),
	}
}

func (g *Game) findQueenMoves(hex Hex) PieceMoves {
	return PieceMoves{
		Piece: g.Board.Get(hex.File, hex.Rank),
		From:  hex,
		Moves: g.findMovesByTraveling(hex, KingOffsets),
	}
}

func (g *Game) findKnightMoves(hex Hex) PieceMoves {
	return PieceMoves{
		Piece: g.Board.Get(hex.File, hex.Rank),
		From:  hex,
		Moves: g.findOffsetMoves(hex, KnightOffsets),
	}
}

func (g *Game) findKingMovesFiltered(hex Hex) PieceMoves {
	return PieceMoves{
		Piece: g.Board.Get(hex.File, hex.Rank),
		From:  hex,
		Moves: g.findOffsetMovesFiltered(hex, KingOffsets),
	}
}

func (g *Game) findPawnMovesWhite(hex Hex) PieceMoves {
	return g.findPawnMoves(hex, true)
}

func (g *Game) findPawnMovesBlack(hex Hex) PieceMoves {
	return g.findPawnMoves(hex, false)
}

func (g *Game) setAttackTable(piece Piece, move Hex) {
	if piece.IsWhite() {
		g.WhiteAttackTable[move.File][move.Rank] = true
	} else {
		g.BlackAttackTable[move.File][move.Rank] = true
	}
}

var (
	WhiteAhead     = []Direction{Up}
	WhiteTakeLeft  = []Direction{UpLeft}
	WhiteTakeRight = []Direction{UpRight}
	BlackAhead     = []Direction{Down}
	BlackTakeLeft  = []Direction{DownLeft}
	BlackTakeRight = []Direction{DownRight}
)

func (g *Game) findPawnMoves(hex Hex, isWhiteTurn bool) PieceMoves {
	pick := func(cond bool, a, b []Direction) []Direction {
		if cond {
			return a
		}
		return b
	}

	basePiece := g.Board.Get(hex.File, hex.Rank)
	var moves []Hex

	move1 := hex.Walk(pick(isWhiteTurn, WhiteAhead, BlackAhead))
	if g.Board.InBoundsHex(move1) && g.Board.Get(move1.File, move1.Rank) == Empty {
		moves = append(moves, move1)
	}

	move2 := move1.Walk(pick(isWhiteTurn, WhiteAhead, BlackAhead))
	if g.Board.InBoundsHex(move2) && !HasPawnMoved(hex, basePiece.IsWhite()) {
		if g.Board.Get(move2.File, move2.Rank) == Empty {
			moves = append(moves, move2)
		}
	}
	move3 := hex.Walk(pick(isWhiteTurn, WhiteTakeLeft, BlackTakeLeft))
	if g.Board.InBoundsHex(move3) {
		piece := g.Board.Get(move3.File, move3.Rank)
		if piece != Empty && basePiece.OppositeColor(piece) {
			moves = append(moves, move3)
			g.setAttackTable(basePiece, move3)
		}
	}

	move4 := hex.Walk(pick(isWhiteTurn, WhiteTakeRight, BlackTakeRight))
	if g.Board.InBoundsHex(move4) {
		piece := g.Board.Get(move4.File, move4.Rank)
		if piece != Empty && basePiece.OppositeColor(piece) {
			moves = append(moves, move4)
			g.setAttackTable(basePiece, move4)
		}
	}

	return PieceMoves{Piece: g.Board.Get(hex.File, hex.Rank), From: hex, Moves: moves}
}

func (g *Game) findMovesByTraveling(hex Hex, directions [][]Direction) []Hex {
	basePiece := g.Board.Get(hex.File, hex.Rank)
	var moves []Hex

	//checkPinned := func(move Hex, dirSeq []Direction) {
	//	for {
	//		move = move.Walk(dirSeq)
	//		if !g.Board.InBoundsHex(move) {
	//			break
	//		}
	//		piece := g.Board.Get(move.File, move.Rank)
	//		if piece.IsKing() && piece.OppositeColor(basePiece) {
	//			// we found the opposing king, so this piece is pinned
	//			break
	//		}
	//	}
	//}

	for _, dirSeq := range directions {
		move := hex
		for {
			move = move.Walk(dirSeq)
			if !g.Board.InBoundsHex(move) {
				break
			}
			piece := g.Board.Get(move.File, move.Rank)
			if piece == Empty {
				moves = append(moves, move)
				g.setAttackTable(basePiece, move)
			} else {
				if piece.OppositeColor(basePiece) {
					moves = append(moves, move)
					g.setAttackTable(basePiece, move)
					//checkPinned(move, dirSeq)
				}
				break
			}
		}
	}

	return moves
}

func (g *Game) findOffsetMoves(hex Hex, directions [][]Direction) []Hex {
	return g.findOffsetMovesFiltered(hex, directions)
}

func (g *Game) findOffsetMovesFiltered(hex Hex, directions [][]Direction) []Hex {
	basePiece := g.Board.Get(hex.File, hex.Rank)
	var moves []Hex

	for _, dirSeq := range directions {
		move := hex.Walk(dirSeq)
		if !g.Board.InBoundsHex(move) {
			continue
		}

		piece := g.Board.Get(move.File, move.Rank)
		canMove := piece == Empty || basePiece.OppositeColor(piece)

		isAttacked := false
		switch basePiece {
		case WhiteKing:
			isAttacked = g.BlackAttackTable[move.File][move.Rank]
		case BlackKing:
			isAttacked = g.WhiteAttackTable[move.File][move.Rank]
		default:
		}
		if canMove && !isAttacked {
			moves = append(moves, move)
			g.setAttackTable(basePiece, move)
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

var ErrKingAssert = errors.New("assertion error: should never be allowed to make a move to the king")

func RandomMoveSeq(game Game, low int, hi int) ([]HistMove, error) {
	for range rand.Intn(low) + (low + hi) {
		game.InitPieceMoves()

		var pmsArr []PieceMoves
		for _, pm := range game.GetCurrMoves() {
			if len(pm.Moves) > 0 {
				pmsArr = append(pmsArr, pm)
			}
		}
		if len(pmsArr) == 0 {
			break
		}

		pms := pmsArr[rand.Intn(len(pmsArr))]
		if len(pms.Moves) == 0 {
			return nil, fmt.Errorf("expected at least one move, got none for game: %v", game)
		}
		pm := PieceMove{
			Piece: game.Board.Get(pms.From.File, pms.From.Rank),
			From:  pms.From,
			To:    pms.Moves[rand.Intn(len(pms.Moves))],
		}

		toPiece := game.Board.Get(pm.To.File, pm.To.Rank)
		fromPiece := game.Board.Get(pm.From.File, pm.From.Rank)
		if toPiece.IsKing() {
			return nil, ErrKingAssert
		}
		if pm.From == pm.To {
			return nil, fmt.Errorf("expected from != to, got %v", pm)
		}
		if toPiece.SameColor(fromPiece) {
			return nil, fmt.Errorf("expected move to not be to piece of same color %v", pm)
		}

		game.MakeMove(pm.From, pm.To)
	}

	slog.Info("random move sequence", "len", len(game.Moves))
	return game.Moves, nil
}
