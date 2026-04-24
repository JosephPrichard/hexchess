package hexchess

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Place struct {
	Not   string
	Piece Piece
}

type Move struct {
	From      Hex
	To        Hex
	Promotion Promotion
}

type AnnotatedMove struct {
	PieceMove
	Promotion Promotion
	CollFile  bool
	CollRank  bool
	IsCheck   bool
	IsTake    bool
}

func (m AnnotatedMove) Equals(m1 AnnotatedMove) bool {
	return m.PieceMove == m1.PieceMove &&
		m.Promotion == m1.Promotion &&
		m.CollFile == m1.CollFile &&
		m.CollRank == m1.CollRank &&
		m.IsCheck == m1.IsCheck &&
		m.IsTake == m1.IsTake
}

func (m AnnotatedMove) String() string {
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
	if m.Promotion != 0 {
		piece := GetPromoPiece(m.Promotion, m.Piece.IsWhite())
		sb.WriteRune('=')
		sb.WriteRune(piece.Rune())
	}

	return sb.String()
}

type HistMove struct {
	PieceMove
	Promotion  Promotion
	Notation   string
	WhiteTimer time.Duration
	BlackTimer time.Duration
}

type Promotion int

const (
	_              = iota
	QueenPromotion = iota
	RookPromotion
	BishopPromotion
	KnightPromotion
)

func GetPromoPiece(promotion Promotion, isWhiteTurn bool) Piece {
	var piece Piece
	if isWhiteTurn {
		switch promotion {
		case RookPromotion:
			piece = WhiteRook
		case BishopPromotion:
			piece = WhiteBishop
		case KnightPromotion:
			piece = WhiteKnight
		default:
			piece = WhiteQueen
		}
	} else {
		switch promotion {
		case RookPromotion:
			piece = BlackRook
		case BishopPromotion:
			piece = BlackBishop
		case KnightPromotion:
			piece = BlackKnight
		default:
			piece = BlackQueen
		}
	}
	return piece
}

type MoveErrorKind int

const (
	MoveErrNoop MoveErrorKind = iota + 1
	MoveErrOutOfBounds
	MoveErrInvalidPromotion
	MoveErrPieceCannotMove
	MoveErrIllegalDestination
	MoveErrIllegalTarget
)

type MoveError struct {
	Kind MoveErrorKind
	Move Move
}

func (e MoveError) Error() string {
	switch e.Kind {
	case MoveErrNoop:
		return fmt.Sprintf("move is a noop: %v", e.Move)
	case MoveErrOutOfBounds:
		return fmt.Sprintf("move is ext of bounds: %v", e.Move)
	case MoveErrInvalidPromotion:
		return fmt.Sprintf("promotion is invalid: %v", e.Move)
	case MoveErrPieceCannotMove:
		return fmt.Sprintf("piece cannot move: %v", e.Move)
	case MoveErrIllegalDestination:
		return fmt.Sprintf("piece cannot move to hex: %v", e.Move)
	case MoveErrIllegalTarget:
		return fmt.Sprintf("cannot move piece at hex: %v", e.Move)
	default:
		return fmt.Sprintf("invalid move: %v", e.Move)
	}
}
