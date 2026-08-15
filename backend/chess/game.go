package chess

import (
	// "encoding/json"
	"fmt"
	"slices"
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
	// keeps track of information while executing InitPieceMoves, zeroed ext every time we calculate moves again
	WhiteAttackTable AttackTable
	BlackAttackTable AttackTable
	PinTable         [Files][MaxRanks][]Hex
}

func NewStartGame(initial ...Place) Game {
	return Game{Board: NewStartBoard(initial...)}
}

func NewEmptyGame(isWhiteTurn bool, initial ...Place) Game {
	return Game{Board: NewEmptyBoard(isWhiteTurn, initial...)}
}

func (g *Game) LastMove() HistMove {
	if len(g.Moves) == 0 {
		panic("tried to retrieve the last move on a game with no moves")
	}
	return g.Moves[len(g.Moves)-1]
}

func (g *Game) SetPieces(initial ...Place) {
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

func (g *Game) NewMoved(mv Move) Game {
	game := g.DeepCopy()
	game.NewMove(mv)
	return game
}

func (g *Game) IsPromotion(mv Move) bool {
	isPawn := g.Board.Get(mv.From.File, mv.From.Rank).IsPawn()
	if !isPawn {
		return false
	}
	if g.Board.IsWhiteTurn {
		return mv.To.File < uint32(len(RanksPerFile)) && mv.To.Rank == RanksPerFile[mv.To.File]-1
	} else {
		return mv.To.Rank == 0
	}
}

func (g *Game) NewMove(move Move) HistMove {
	// preconditions: to and from are valid locations on the board, promotion is a valid promotion
	from := move.From
	to := move.To
	promotion := move.Promotion

	pieceFrom := g.Board.Get(from.File, from.Rank)
	pieceTo := g.Board.Get(to.File, to.Rank)
	isPromotion := g.IsPromotion(move)

	g.Board.Set(from.File, from.Rank, Empty)
	if isPromotion {
		if promotion == 0 {
			promotion = QueenPromotion
		}
		g.Board.Set(to.File, to.Rank, GetPromoPiece(promotion, g.Board.IsWhiteTurn))
	} else {
		g.Board.Set(to.File, to.Rank, pieceFrom)
	}

	annotMove := g.NewAnnotatedMove(PieceMove{Piece: pieceFrom, From: from, To: to})
	if isPromotion {
		annotMove.Promotion = promotion
	}

	g.Board.IsWhiteTurn = !g.Board.IsWhiteTurn

	if pieceTo != Empty {
		if g.Board.IsWhiteTurn {
			g.TakenWhitePieces = append(g.TakenWhitePieces, pieceTo)
		} else {
			g.TakenBlackPieces = append(g.TakenBlackPieces, pieceTo)
		}
	}

	g.WhiteMoves = nil
	g.BlackMoves = nil

	return HistMove{
		PieceMove: annotMove.PieceMove,
		Promotion: annotMove.Promotion,
		Notation:  annotMove.String(),
	}
}

func (g *Game) ValidateMove(move Move) error {
	if move.From.File == move.To.File && move.From.Rank == move.To.Rank {
		return MoveError{Kind: MoveErrNoop, Move: move}
	}
	if !g.Board.InBoundsHex(move.From) || !g.Board.InBoundsHex(move.To) {
		return MoveError{Kind: MoveErrOutOfBounds, Move: move}
	}
	if g.IsPromotion(move) {
		switch move.Promotion {
		case QueenPromotion, RookPromotion, BishopPromotion, KnightPromotion:
		default:
			return MoveError{Kind: MoveErrInvalidPromotion, Move: move}
		}
	}
	legalMoves := g.GetCurrMoves()
	pmsIdx := slices.IndexFunc(legalMoves, func(moves PieceMoves) bool { return moves.From == move.From })
	if pmsIdx < 0 {
		return MoveError{Kind: MoveErrIllegalTarget, Move: move}
	}
	if moveIdx := slices.IndexFunc(legalMoves[pmsIdx].Moves, func(to Hex) bool { return to == move.To }); moveIdx < 0 {
		return MoveError{Kind: MoveErrIllegalDestination, Move: move}
	}
	return nil
}

func (g *Game) NewValidMove(move Move) (HistMove, error) {
	if err := g.ValidateMove(move); err != nil {
		return HistMove{}, err
	}
	hm := g.NewMove(move)
	g.Moves = append(g.Moves, hm)
	g.InitPieceMoves()
	return hm, nil
}

func (g *Game) NewHistMove(move Move) {
	g.Moves = append(g.Moves, g.NewMove(move))
}

func (g *Game) NewAnnotatedMove(pm PieceMove) AnnotatedMove {
	hm := AnnotatedMove{PieceMove: pm}

	movingPiece := hm.Piece
	moveTo := hm.To
	moveFrom := hm.From

	boardPiece := g.Board.Get(hm.To.File, hm.To.Rank)
	if boardPiece != Empty && boardPiece.IsWhite() != movingPiece.IsWhite() {
		hm.IsTake = true
	}

	moves := g.GetCurrMoves()
	if index := slices.IndexFunc(moves, func(pms PieceMoves) bool { return pms.Piece == movingPiece }); index > 0 {
		pieceMoves := moves[index]
		for _, move := range pieceMoves.Moves {
			target := g.Board.Get(move.File, move.Rank)

			isCheckingBlack := movingPiece.IsWhite() && target == BlackKing
			isCheckingWhite := !movingPiece.IsWhite() && target == WhiteKing

			if isCheckingBlack || isCheckingWhite {
				hm.IsCheck = true
				break
			}
		}
	}

	for _, pms := range moves {
		if pms.Piece != movingPiece {
			continue
		}
		if slices.ContainsFunc(pms.Moves, func(to Hex) bool { return to == moveTo }) {
			if pms.From.File == moveFrom.File {
				hm.CollFile = true
			}
			if pms.From.Rank == moveFrom.Rank {
				hm.CollRank = true
			}
		}
	}
	return hm
}

func (g *Game) EnsurePieceMoves() {
	// initializes the piece moves only if they have not been assigned.
	if g.BlackMoves == nil || g.WhiteMoves == nil {
		g.InitPieceMoves()
	}
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
	moves := make([]PieceMoves, 0)
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
	moves := make([]Hex, 0)

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
	moves := make([]Hex, 0)

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
	moves := make([]Hex, 0)

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

func ApplyMoveSeq(moves ...Move) []HistMove {
	game := NewStartGame()

	for _, m := range moves {
		game.Moves = append(game.Moves, game.NewMove(m))
	}

	return game.Moves
}

type JumpIndexError struct {
	Count int
	Len   int
}

func (e JumpIndexError) Error() string {
	return fmt.Sprintf("invalid rewind offset, must be between 0 and move length (%d), got: %d", e.Len, e.Count)
}

func JumpMoveIndex(initial Board, moves []HistMove, index int) (*Game, error) {
	count := index + 1 // requesting chess 0 means reapplying 1 move.

	if count < 0 || count > len(moves) {
		return nil, JumpIndexError{Count: count, Len: len(moves)}
	}
	undoGame := &Game{Board: initial}
	movesExceptLast := moves[:count]
	for _, move := range movesExceptLast {
		_ = undoGame.NewMove(Move{From: move.From, To: move.To, Promotion: move.Promotion})
		// redoing the move has recomputed the hist move. this should be the same as the original move
		//if !redoMove.Equals(move) {
		//	return nil, fmt.Errorf("expected redoMove == move, got %#v != %#v", redoMove, move)
		//}
		undoGame.Moves = append(undoGame.Moves, move)
	}
	undoGame.InitPieceMoves()
	return undoGame, nil
}
