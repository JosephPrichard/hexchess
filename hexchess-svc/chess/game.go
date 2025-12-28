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

type NotMove struct {
	Not   string
	Piece Piece
}

func MakeStartGame(initial ...NotMove) Game {
	return Game{Board: MakeStartBoard(initial...)}
}

func MakeEmptyGame(isWhiteTurn bool, initial ...NotMove) Game {
	return Game{Board: MakeEmptyBoard(isWhiteTurn, initial...)}
}

func (g *Game) SetPieces(initial ...NotMove) {
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

type Promotion int

const (
	_ = iota
	QueenPromotion
	RookPromotion
	BishopPromotion
	KnightPromotion
)

func IsLastRank(to Hex) bool {
	return to.File < uint32(len(RanksPerFile)) && to.Rank == RanksPerFile[to.File]-1
}

func GetPromoPiece(promotion Promotion, isWhiteTurn bool) Piece {
	piecePromo := Empty
	switch promotion {
	case QueenPromotion:
		piecePromo = WhiteQueen
	case RookPromotion:
		piecePromo = WhiteRook
	case BishopPromotion:
		piecePromo = WhiteBishop
	case KnightPromotion:
		piecePromo = WhiteKnight
	}
	if !isWhiteTurn && piecePromo != Empty {
		piecePromo += 1
	}
	return piecePromo
}

type Move struct {
	From      Hex
	To        Hex
	Promotion Promotion
}

func (g *Game) MakeMoved(mv Move) Game {
	game := g.DeepCopy()
	game.MakeMove(mv)
	return game
}

func (g *Game) MakeMove(mv Move) HistMove {
	// preconditions: to and from are valid locations on the board, promotion is a valid promotion
	from := mv.From
	to := mv.To
	promotion := mv.Promotion
	isMvLastRank := IsLastRank(mv.To)

	pieceFrom := g.Board.Get(from.File, from.Rank)
	pieceTo := g.Board.Get(to.File, to.Rank)

	if pieceTo != Empty {
		if g.Board.IsWhiteTurn {
			g.TakenWhitePieces = append(g.TakenWhitePieces, pieceTo)
		} else {
			g.TakenBlackPieces = append(g.TakenBlackPieces, pieceTo)
		}
	}

	hm := g.AnnotateHistMove(HistMove{PieceMove: PieceMove{Piece: pieceFrom, From: from, To: to}})

	g.Board.Set(from.File, from.Rank, Empty)
	if isMvLastRank {
		promoPiece := GetPromoPiece(promotion, g.Board.IsWhiteTurn)
		g.Board.Set(to.File, to.Rank, promoPiece)
		hm.Promotion = promotion
	} else {
		g.Board.Set(to.File, to.Rank, pieceFrom)
	}
	g.Board.IsWhiteTurn = !g.Board.IsWhiteTurn

	g.Moves = append(g.Moves, hm)
	return hm
}

func (g *Game) ValidateMove(move Move) error {
	if move.From.File == move.To.File && move.From.Rank == move.To.Rank {
		return fmt.Errorf("move is a noop: %v", move)
	}
	if !g.Board.InBoundsHex(move.From) || !g.Board.InBoundsHex(move.To) {
		return fmt.Errorf("move is out of bounds: %v", move)
	}
	if IsLastRank(move.To) && g.Board.Get(move.From.File, move.From.Rank).IsPawn() {
		switch move.Promotion {
		case QueenPromotion, RookPromotion, BishopPromotion, KnightPromotion:
		default:
			return fmt.Errorf("promotion is invalid %v", move)
		}
	}
	legalMoves := g.GetCurrMoves()
	pmsIdx := slices.IndexFunc(legalMoves, func(moves PieceMoves) bool { return moves.From == move.From })
	if pmsIdx < 0 {
		return fmt.Errorf("piece cannot move: %v", move)
	}
	if moveIdx := slices.IndexFunc(legalMoves[pmsIdx].Moves, func(to Hex) bool { return to == move.To }); moveIdx < 0 {
		return fmt.Errorf("piece cannot move to hex: %v", move)
	}
	return nil
}

type HistMove struct {
	// stores any information necessary to generate a move history notation
	PieceMove
	Promotion Promotion
	CollFile  bool
	CollRank  bool
	IsCheck   bool
	IsTake    bool
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
			sb.WriteString(strconv.Itoa(int(from.Rank + 1)))
		}
		if m.CollRank {
			sb.WriteRune(rune(from.File + 'a'))
		}
	}

	sb.WriteString(to.String())
	piece := GetPromoPiece(m.Promotion, m.Piece.IsWhite())
	if piece != Empty {
		sb.WriteRune('=')
		sb.WriteRune(piece.Rune())
	}

	return sb.String()
}

func (g *Game) AnnotateHistMove(hm HistMove) HistMove {
	if g.Board.Get(hm.To.File, hm.To.Rank) != Empty {
		hm.IsTake = true
	}

	moves := g.GetCurrMoves()
	if index := slices.IndexFunc(moves, func(pms PieceMoves) bool { return pms.Piece == hm.Piece }); index > 0 {
		for _, h := range moves[index].Moves {
			p := g.Board.Get(h.File, h.Rank)
			if p.IsKing() {
				hm.IsCheck = true
				break
			}
		}
	}

	for _, pms := range moves {
		if pms.Piece != hm.Piece {
			continue
		}
		if slices.ContainsFunc(pms.Moves, func(to Hex) bool { return to == hm.To }) {
			if pms.From.File == hm.From.File {
				hm.CollFile = true
			}
			if pms.From.Rank == hm.From.Rank {
				hm.CollRank = true
			}
		}
	}
	return hm
}

func (g *Game) InitPieceMoves() {
	g.ClearTables()

	g.WhiteMoves = g.findPieceMoves(true)
	g.BlackMoves = g.findPieceMoves(false)

	whiteKingHex, hasWk := g.Board.FindKing(true)
	blackKingHex, hasBk := g.Board.FindKing(false)

	handleCheck := func(kingHex Hex) {
		isAttacked := g.GetTurnAttackTable(g.Board.IsWhiteTurn)
		isCheck := isAttacked[kingHex.File][kingHex.Rank]
		if isCheck {
			// TODO: add support for maintaining all "blocking" moves
			g.clearCurrentMoves()
		}
	}
	if g.Board.IsWhiteTurn && hasWk {
		handleCheck(whiteKingHex)
	} else if hasBk {
		handleCheck(blackKingHex)
	}

	if hasWk {
		g.WhiteMoves = append(g.WhiteMoves, g.findKingMovesFiltered(whiteKingHex))
	}
	if hasBk {
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

		game.MakeMove(Move{From: pm.From, To: pm.To, Promotion: QueenPromotion})
	}

	slog.Info("random move sequence", "len", len(game.Moves))
	return game.Moves, nil
}
