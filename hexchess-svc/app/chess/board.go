package chess

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Piece byte

const (
	Empty Piece = iota
	WhitePawn
	BlackPawn
	WhiteKnight
	BlackKnight
	WhiteBishop
	BlackBishop
	WhiteRook
	BlackRook
	WhiteQueen
	BlackQueen
	WhiteKing
	BlackKing

	MaxRanks = 11
	Files    = 11
)

var RanksPerFile = []int{6, 7, 8, 9, 10, 11, 10, 9, 8, 7, 6}
var OrdHexagons = GetOrdHexagons()

func GetOrdHexagons() []Hexagon {
	var hexagons []Hexagon
	for file := range Files {
		for rank := range RanksPerFile[file] {
			hexagons = append(hexagons, Hexagon{File: file, Rank: rank})
		}
	}
	return hexagons
}

const Midpoint = 5

type Direction int

const (
	Up Direction = iota
	Down
	DownLeft
	DownRight
	UpLeft
	UpRight
)

var RookOffsets = [][]Direction{
	{Up},
	{Down},
	{DownLeft},
	{DownRight},
	{UpLeft},
	{UpRight},
}
var BishopOffsets = [][]Direction{
	{UpRight, DownRight},
	{UpLeft, DownLeft},
	{Up, UpRight},
	{Up, UpLeft},
	{Down, DownRight},
	{Down, DownLeft},
}
var KingOffsets = append(RookOffsets, BishopOffsets...)
var KnightOffsets = [][]Direction{
	{UpRight, UpRight, Up},
	{UpRight, Up, Up},
	{DownRight, DownRight, Down},
	{DownRight, Down, Down},
	{UpLeft, UpLeft, Up},
	{UpLeft, Up, Up},
	{DownLeft, DownLeft, Down},
	{DownLeft, Down, Down},
	{UpLeft, UpLeft, DownLeft},
	{DownLeft, DownLeft, UpLeft},
	{UpRight, UpRight, DownRight},
	{DownRight, DownRight, UpRight},
}

type Hexagon struct {
	File int
	Rank int
}

type PieceMoves struct {
	From  Hexagon
	Moves []Hexagon
}

type PieceMove struct {
	Piece Piece
	From  Hexagon
	To    Hexagon
}

type Board struct {
	IsWhiteTurn bool
	Pieces      [Files][]Piece
}

func ParseHexagon(notation string) (Hexagon, error) {
	file := int(notation[0] - 'a')
	rank, err := strconv.Atoi(notation[1:])
	if err != nil {
		return Hexagon{}, fmt.Errorf("failed to parse hexagon: %w", err)
	}
	return Hexagon{File: file, Rank: rank - 1}, nil
}

func ParseHexagonValid(notation string) Hexagon {
	hex, err := ParseHexagon(notation)
	if err != nil {
		panic(fmt.Sprintf("failed to set piece at notation: %v", err))
	}
	return hex
}

func (h Hexagon) String() string {
	// String returns a string like "a1" from a Hexagon
	fileChar := rune(h.File + 'a')
	return fmt.Sprintf("%c%d", fileChar, h.Rank+1)
}

func (h Hexagon) CanPromote() bool {
	switch h.File {
	case 0, 10:
		return h.Rank >= 5
	case 1, 9:
		return h.Rank >= 6
	case 2, 8:
		return h.Rank >= 7
	case 3, 7:
		return h.Rank >= 8
	case 4, 6:
		return h.Rank >= 9
	case 5:
		return h.Rank >= 10
	default:
		panic(fmt.Sprintf("cannot promote to an invalid file %d", h.File))
	}
	return false
}

func (h Hexagon) Walk(directions []Direction) Hexagon {
	file := h.File
	rank := h.Rank
	for _, direction := range directions {
		switch direction {
		case Up:
			rank += 1
		case Down:
			rank -= 1
		case UpLeft:
			if file > Midpoint {
				rank += 1
			}
			file -= 1
		case DownLeft:
			if file <= Midpoint {
				rank -= 1
			}
			file -= 1
		case UpRight:
			if file < Midpoint {
				rank += 1
			}
			file += 1
		case DownRight:
			if file >= Midpoint {
				rank -= 1
			}
			file += 1
		}
	}
	return Hexagon{File: file, Rank: rank}
}

func (b *Board) FindKing(isWhiteTurn bool) Hexagon {
	for _, hex := range OrdHexagons {
		piece := b.Pieces[hex.File][hex.Rank]
		if (piece == WhiteKing && isWhiteTurn) || (piece == BlackKing && !isWhiteTurn) {
			return hex
		}
	}
	panic("board doesn't have a king")
	return Hexagon{}
}

func HasPawnMoved(pawnHex Hexagon, isWhite bool) bool {
	if isWhite {
		var minRank int
		switch pawnHex.File {
		case 1, 9:
			minRank = 0
		case 2, 8:
			minRank = 1
		case 3, 7:
			minRank = 2
		case 4, 6:
			minRank = 3
		case 5:
			minRank = 4
		default:
			minRank = -1 // any other file: pawn must have moved
		}
		return pawnHex.Rank > minRank
	} else {
		return pawnHex.Rank < 7
	}
}

func (piece Piece) String() string {
	return string(piece.ToChar())
}

func (piece Piece) IsPieceTurn(isWhiteTurn bool) bool {
	return (piece%2 == 0 && !isWhiteTurn) || (piece%2 == 1 && isWhiteTurn)
}

func (piece Piece) IsWhite() bool {
	return piece%2 == 1
}

func AreOpposite(piece1, piece2 Piece) bool {
	if piece1 == Empty || piece2 == Empty {
		panic(fmt.Sprintf("pieces are empty when checking if opposite, was piece1=%v and piece2=%v", piece1, piece2))
	}
	return piece1%2 != piece2%2
}

func (piece Piece) ToChar() rune {
	switch piece {
	case Empty:
		return '.'
	case WhitePawn:
		return 'P'
	case BlackPawn:
		return 'p'
	case WhiteKnight:
		return 'N'
	case BlackKnight:
		return 'n'
	case WhiteBishop:
		return 'B'
	case BlackBishop:
		return 'b'
	case WhiteRook:
		return 'R'
	case BlackRook:
		return 'r'
	case WhiteQueen:
		return 'Q'
	case BlackQueen:
		return 'q'
	case WhiteKing:
		return 'K'
	case BlackKing:
		return 'k'
	default:
		panic(fmt.Sprintf("invalid piece: %d", piece))
	}
	return 0
}

func (piece Piece) IsPawn() bool {
	return piece == WhitePawn || piece == BlackPawn
}

func (piece Piece) IsKing() bool {
	return piece == WhiteKing || piece == BlackKing
}

func MakeBoard(isWhiteTurn bool) Board {
	b := Board{IsWhiteTurn: isWhiteTurn}
	for i := range b.Pieces {
		b.Pieces[i] = make([]Piece, RanksPerFile[i])
	}
	return b
}

func InitialBoard() Board {
	board := MakeBoard(true)

	board.SetPiece("b1", WhitePawn)
	board.SetPiece("c2", WhitePawn)
	board.SetPiece("d3", WhitePawn)
	board.SetPiece("e4", WhitePawn)
	board.SetPiece("f5", WhitePawn)
	board.SetPiece("g4", WhitePawn)
	board.SetPiece("h3", WhitePawn)
	board.SetPiece("i2", WhitePawn)
	board.SetPiece("j1", WhitePawn)

	board.SetPiece("c1", WhiteRook)
	board.SetPiece("d1", WhiteKnight)
	board.SetPiece("e1", WhiteQueen)
	board.SetPiece("f1", WhiteBishop)
	board.SetPiece("f2", WhiteBishop)
	board.SetPiece("f3", WhiteBishop)
	board.SetPiece("g1", WhiteKing)
	board.SetPiece("h1", WhiteKnight)
	board.SetPiece("i1", WhiteRook)

	board.SetPiece("b7", BlackPawn)
	board.SetPiece("c7", BlackPawn)
	board.SetPiece("d7", BlackPawn)
	board.SetPiece("e7", BlackPawn)
	board.SetPiece("f7", BlackPawn)
	board.SetPiece("g7", BlackPawn)
	board.SetPiece("h7", BlackPawn)
	board.SetPiece("i7", BlackPawn)
	board.SetPiece("j7", BlackPawn)

	board.SetPiece("c8", BlackRook)
	board.SetPiece("d9", BlackKnight)
	board.SetPiece("e10", BlackQueen)
	board.SetPiece("f11", BlackBishop)
	board.SetPiece("f10", BlackBishop)
	board.SetPiece("f9", BlackBishop)
	board.SetPiece("g10", BlackKing)
	board.SetPiece("h9", BlackKnight)
	board.SetPiece("i8", BlackRook)

	return board
}

func (b *Board) SetPiece(str string, p Piece) {
	hex := ParseHexagonValid(str)
	b.Pieces[hex.File][hex.Rank] = p
}

func (b *Board) GetPiece(str string) Piece {
	hex := ParseHexagonValid(str)
	return b.Pieces[hex.File][hex.Rank]
}

func (b *Board) InBounds(file, rank int) bool {
	if file < 0 || file >= Files || rank < 0 {
		return false
	}
	return rank < len(b.Pieces[file])
}

func (b *Board) InBoundsHex(hex Hexagon) bool {
	return b.InBounds(hex.File, hex.Rank)
}

func (b *Board) ToPieceMovesString(moves []PieceMoves) string {
	var isAttacked [Files][]bool
	for i := range len(isAttacked) {
		isAttacked[i] = make([]bool, RanksPerFile[i])
	}

	for _, pm := range moves {
		for _, move := range pm.Moves {
			isAttacked[move.File][move.Rank] = true
		}
	}

	return b.StringFunc(func(hex Hexagon) bool {
		return isAttacked[hex.File][hex.Rank]
	})
}

func (b *Board) String() string {
	// default: no moves highlighted
	return b.StringFunc(func(h Hexagon) bool { return false })
}

func (b *Board) StringFunc(isMove func(Hexagon) bool) string {
	var sb strings.Builder
	sb.WriteString("\n")

	for file := 0; file < Files; file++ {
		ranksCount := RanksPerFile[file]
		ranksDiff := MaxRanks - ranksCount

		fileChar := rune(file + 'a')
		sb.WriteRune(fileChar)
		sb.WriteString("   ")

		for i := 0; i < ranksDiff; i++ {
			sb.WriteString("  ")
		}

		for rank := 0; rank < ranksCount; rank++ {
			hex := Hexagon{File: file, Rank: rank}
			if isMove(hex) {
				sb.WriteString("x   ")
			} else {
				piece := b.Pieces[file][rank]
				sb.WriteRune(piece.ToChar())
				sb.WriteString("   ")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func (b *Board) ToMovesString(moves []Hexagon) string {
	return b.StringFunc(func(hex Hexagon) bool {
		return slices.Contains(moves, hex)
	})
}
