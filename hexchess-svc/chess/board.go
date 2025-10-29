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

func GetOrdHexagons() []Hex {
	var hexagons []Hex
	for file := range Files {
		for rank := range RanksPerFile[file] {
			hexagons = append(hexagons, Hex{File: file, Rank: rank})
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

type Hex struct {
	File int `json:"file"`
	Rank int `json:"rank"`
}

type PieceMoves struct {
	Piece Piece `json:"piece"`
	From  Hex   `json:"from"`
	Moves []Hex `json:"moves"`
}

type PieceMove struct {
	Piece Piece `json:"piece"`
	From  Hex   `json:"from"`
	To    Hex   `json:"to"`
}

type Board struct {
	IsWhiteTurn bool
	Pieces      [Files][MaxRanks]Piece // over allocated to keep the array packed within the struct
}

func ParseHexagon(notation string) (Hex, error) {
	file := int(notation[0] - 'a')
	rank, err := strconv.Atoi(notation[1:])
	if err != nil {
		return Hex{}, fmt.Errorf("failed to parse hexagon: %w", err)
	}
	return Hex{File: file, Rank: rank - 1}, nil
}

func ParseHexagonValid(notation string) Hex {
	hex, err := ParseHexagon(notation)
	if err != nil {
		panic(fmt.Sprintf("failed to set piece at notation: %v", err))
	}
	return hex
}

func (h Hex) String() string {
	// String returns a string like "a1" from a Hexagon
	fileChar := rune(h.File + 'a')
	return fmt.Sprintf("%c%d", fileChar, h.Rank+1)
}

func (h Hex) CanPromote() bool {
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

func (h Hex) Walk(directions []Direction) Hex {
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
	return Hex{File: file, Rank: rank}
}

func HasPawnMoved(pawnHex Hex, isWhite bool) bool {
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

func (p Piece) String() string {
	return string(p.ToChar())
}

func (p Piece) IsPieceTurn(isWhiteTurn bool) bool {
	return (p%2 == 0 && !isWhiteTurn) || (p%2 == 1 && isWhiteTurn)
}

func (p Piece) IsWhite() bool {
	return p%2 == 1
}

func AreOpposite(piece1, piece2 Piece) bool {
	if piece1 == Empty || piece2 == Empty {
		panic(fmt.Sprintf("pieces are empty when checking if opposite, was piece1=%v and piece2=%v", piece1, piece2))
	}
	return piece1%2 != piece2%2
}

func (p Piece) ToChar() rune {
	switch p {
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
		panic(fmt.Sprintf("invalid piece: %d", p))
	}
	return 0
}

func (p Piece) IsPawn() bool {
	return p == WhitePawn || p == BlackPawn
}

func (p Piece) IsKing() bool {
	return p == WhiteKing || p == BlackKing
}

func MakeBoard(isWhiteTurn bool) Board {
	b := Board{IsWhiteTurn: isWhiteTurn}
	//for i := range b.Pieces {
	//	b.Pieces[i] = make([]Piece, RanksPerFile[i])
	//}
	return b
}

func InitialBoard() Board {
	board := MakeBoard(true)

	board.SetPieceNot("b1", WhitePawn)
	board.SetPieceNot("c2", WhitePawn)
	board.SetPieceNot("d3", WhitePawn)
	board.SetPieceNot("e4", WhitePawn)
	board.SetPieceNot("f5", WhitePawn)
	board.SetPieceNot("g4", WhitePawn)
	board.SetPieceNot("h3", WhitePawn)
	board.SetPieceNot("i2", WhitePawn)
	board.SetPieceNot("j1", WhitePawn)

	board.SetPieceNot("c1", WhiteRook)
	board.SetPieceNot("d1", WhiteKnight)
	board.SetPieceNot("e1", WhiteQueen)
	board.SetPieceNot("f1", WhiteBishop)
	board.SetPieceNot("f2", WhiteBishop)
	board.SetPieceNot("f3", WhiteBishop)
	board.SetPieceNot("g1", WhiteKing)
	board.SetPieceNot("h1", WhiteKnight)
	board.SetPieceNot("i1", WhiteRook)

	board.SetPieceNot("b7", BlackPawn)
	board.SetPieceNot("c7", BlackPawn)
	board.SetPieceNot("d7", BlackPawn)
	board.SetPieceNot("e7", BlackPawn)
	board.SetPieceNot("f7", BlackPawn)
	board.SetPieceNot("g7", BlackPawn)
	board.SetPieceNot("h7", BlackPawn)
	board.SetPieceNot("i7", BlackPawn)
	board.SetPieceNot("j7", BlackPawn)

	board.SetPieceNot("c8", BlackRook)
	board.SetPieceNot("d9", BlackKnight)
	board.SetPieceNot("e10", BlackQueen)
	board.SetPieceNot("f11", BlackBishop)
	board.SetPieceNot("f10", BlackBishop)
	board.SetPieceNot("f9", BlackBishop)
	board.SetPieceNot("g10", BlackKing)
	board.SetPieceNot("h9", BlackKnight)
	board.SetPieceNot("i8", BlackRook)

	return board
}

//func (b *Board) DeepCopy() Board {
//	board := MakeBoard(b.IsWhiteTurn)
//	for i := range b.Pieces {
//		for j := range b.Pieces[i] {
//			board.Pieces[i][j] = b.Pieces[i][j]
//		}
//	}
//	return board
//}

func (b *Board) SetPiece(file, rank int, piece Piece) error {
	if file >= len(b.Pieces) {
		return fmt.Errorf("file out of range: %d", file)
	}
	fileArr := &b.Pieces[file]
	if rank >= RanksPerFile[file] {
		return fmt.Errorf("rank out of range: %d for file: %d", rank, file)
	}
	fileArr[rank] = piece
	return nil
}

func (b *Board) GetPiece(file, rank int) (Piece, error) {
	if file >= len(b.Pieces) {
		return 0, fmt.Errorf("file out of range: %d", file)
	}
	fileArr := &b.Pieces[file]
	if rank >= RanksPerFile[file] {
		return 0, fmt.Errorf("rank out of range: %d for file: %d", rank, file)
	}
	return fileArr[rank], nil
}

// SetPieceNot GetPieceNot set piece notation, get piece notation
func (b *Board) SetPieceNot(str string, p Piece) {
	hex := ParseHexagonValid(str)
	b.Pieces[hex.File][hex.Rank] = p
}

func (b *Board) GetPieceNot(str string) Piece {
	hex := ParseHexagonValid(str)
	return b.Pieces[hex.File][hex.Rank]
}

func (b *Board) FindKing(isWhiteTurn bool) Hex {
	for _, hex := range OrdHexagons {
		piece := b.Pieces[hex.File][hex.Rank]
		if (piece == WhiteKing && isWhiteTurn) || (piece == BlackKing && !isWhiteTurn) {
			return hex
		}
	}
	panic("board doesn't have a king")
	return Hex{}
}

func (b *Board) InBounds(file, rank int) bool {
	if file < 0 || file >= Files || rank < 0 {
		return false
	}
	return rank < RanksPerFile[file]
}

func (b *Board) InBoundsHex(hex Hex) bool {
	return b.InBounds(hex.File, hex.Rank)
}

func (b *Board) String() string {
	// default: no moves highlighted
	return b.StringFunc(func(h Hex) rune { return 0 })
}

func (b *Board) StringFunc(isMove func(Hex) rune) string {
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
			hex := Hex{File: file, Rank: rank}
			ch := isMove(hex)
			piece := b.Pieces[file][rank]
			if ch != 0 && piece == Empty {
				sb.WriteRune(ch)
			} else {
				sb.WriteRune(piece.ToChar())
			}
			sb.WriteString("   ")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func (b *Board) StringMoves(moves []Hex) string {
	return b.StringFunc(func(hex Hex) rune {
		if slices.Contains(moves, hex) {
			return 'x'
		} else {
			return 0
		}
	})
}
