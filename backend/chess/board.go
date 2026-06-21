package chess

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"hexchess-svc/pb"
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

	MaxRanks uint32 = 11
	Files    uint32 = 11
)

var RanksPerFile = []uint32{6, 7, 8, 9, 10, 11, 10, 9, 8, 7, 6}
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
	File uint32 `json:"file"`
	Rank uint32 `json:"rank"`
}

type PieceMoves struct {
	Piece Piece `json:"piece"`
	From  Hex   `json:"from"`
	Moves []Hex `json:"moves"`
}

func (pms *PieceMoves) DeepCopy() PieceMoves {
	return PieceMoves{
		Piece: pms.Piece,
		From:  pms.From,
		Moves: append([]Hex{}, pms.Moves...),
	}
}

type PieceMove struct {
	Piece Piece `json:"piece"`
	From  Hex   `json:"from"`
	To    Hex   `json:"to"`
}

type BoardMatrix [][]int

type Board struct {
	IsWhiteTurn bool
	Pieces      [Files][MaxRanks]Piece // over allocated to keep the array packed within the struct
}

func (b *Board) ToMatrix() BoardMatrix {
	matrix := make([][]int, 0, Files)
	for i, row := range b.Pieces {
		matrixRow := make([]int, 0, RanksPerFile[i])
		for _, piece := range row {
			matrixRow = append(matrixRow, int(piece))
		}
		matrix = append(matrix, matrixRow)
	}
	return matrix
}

func ParseHexagon(notation string) (Hex, error) {
	n1 := unicode.ToLower(rune(notation[0]))
	n2 := strings.ToLower(notation[1:])
	file := int(n1 - 'a')
	rank, err := strconv.Atoi(n2)
	if err != nil {
		return Hex{}, fmt.Errorf("parse hexagon: %w", err)
	}
	return Hex{File: uint32(file), Rank: uint32(rank - 1)}, nil
}

func HexStr(notation string) Hex {
	hex, err := ParseHexagon(notation)
	if err != nil {
		panic(err)
	}
	return hex
}

func MoveStr(from string, to string) Move {
	return Move{From: HexStr(from), To: HexStr(to)}
}

func PbMoveStr(from string, to string) *pb.Move {
	fromHex := HexStr(from)
	toHex := HexStr(to)
	return &pb.Move{
		FromFile: int32(fromHex.File),
		FromRank: int32(fromHex.Rank),
		ToFile:   int32(toHex.File),
		ToRank:   int32(toHex.Rank),
	}
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
		return int(pawnHex.Rank) > minRank
	} else {
		return pawnHex.Rank < 7
	}
}

func (p Piece) String() string {
	return string(p.Rune())
}

func (p Piece) IsPieceTurn(isWhiteTurn bool) bool {
	return (p%2 == 0 && !isWhiteTurn) || (p%2 == 1 && isWhiteTurn)
}

func (p Piece) IsWhite() bool {
	return p%2 == 1
}

func (p Piece) SameColor(p2 Piece) bool {
	return p != Empty && p2 != Empty && p%2 == p2%2
}

func (p Piece) OppositeColor(p2 Piece) bool {
	return p%2 != p2%2
}

func (p Piece) Rune() rune {
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
		return '?'
	}
}

func PieceFromRune(r rune) (Piece, error) {
	var p Piece
	switch r {
	case '.':
		p = Empty
	case 'P':
		p = WhitePawn
	case 'p':
		p = BlackPawn
	case 'N':
		p = WhiteKnight
	case 'n':
		p = BlackKnight
	case 'B':
		p = WhiteBishop
	case 'b':
		p = BlackBishop
	case 'R':
		p = WhiteRook
	case 'r':
		p = BlackRook
	case 'Q':
		p = WhiteQueen
	case 'q':
		p = BlackQueen
	case 'K':
		p = WhiteKing
	case 'k':
		p = BlackKing
	default:
		return 0, errors.New("invalid piece move")
	}
	return p, nil
}

func (p Piece) IsPawn() bool {
	return p == WhitePawn || p == BlackPawn
}

func (p Piece) IsKing() bool {
	return p == WhiteKing || p == BlackKing
}

var GlobalInitialBoard = NewInitialBoard()

func InitialBoard() Board {
	return GlobalInitialBoard
}

func NewInitialBoard() Board {
	board := Board{IsWhiteTurn: true}

	placements := []struct {
		square string
		piece  Piece
	}{
		// White pawns
		{"b1", WhitePawn},
		{"c2", WhitePawn},
		{"d3", WhitePawn},
		{"e4", WhitePawn},
		{"f5", WhitePawn},
		{"g4", WhitePawn},
		{"h3", WhitePawn},
		{"i2", WhitePawn},
		{"j1", WhitePawn},
		// White back rank
		{"c1", WhiteRook},
		{"d1", WhiteKnight},
		{"e1", WhiteQueen},
		{"f1", WhiteBishop},
		{"f2", WhiteBishop},
		{"f3", WhiteBishop},
		{"g1", WhiteKing},
		{"h1", WhiteKnight},
		{"i1", WhiteRook},
		// Black pawns
		{"b7", BlackPawn},
		{"c7", BlackPawn},
		{"d7", BlackPawn},
		{"e7", BlackPawn},
		{"f7", BlackPawn},
		{"g7", BlackPawn},
		{"h7", BlackPawn},
		{"i7", BlackPawn},
		{"j7", BlackPawn},
		// Black back rank
		{"c8", BlackRook},
		{"d9", BlackKnight},
		{"e10", BlackQueen},
		{"f11", BlackBishop},
		{"f10", BlackBishop},
		{"f9", BlackBishop},
		{"g10", BlackKing},
		{"h9", BlackKnight},
		{"i8", BlackRook},
	}
	for _, p := range placements {
		board.SetPieceNot(p.square, p.piece)
	}

	return board
}

func NewStartBoard(initial ...Place) Board {
	board := InitialBoard()
	for _, pm := range initial {
		board.SetPieceNot(pm.Not, pm.Piece)
	}
	return board
}

func NewEmptyBoard(isWhiteTurn bool, initial ...Place) Board {
	board := Board{IsWhiteTurn: isWhiteTurn}
	for _, pm := range initial {
		board.SetPieceNot(pm.Not, pm.Piece)
	}
	return board
}

func (b *Board) Get(file, rank uint32) Piece {
	return b.Pieces[file][rank]
}

func (b *Board) Set(file, rank uint32, p Piece) {
	b.Pieces[file][rank] = p
}

func (b *Board) SetPiece(file, rank uint32, piece Piece) error {
	if file >= uint32(len(b.Pieces)) {
		return fmt.Errorf("file of range: %d", file)
	}
	fileArr := &b.Pieces[file]
	if rank >= RanksPerFile[file] {
		return fmt.Errorf("rank of range: %d for file: %d", rank, file)
	}
	if piece.Rune() == '?' {
		return fmt.Errorf("unknown piece type: %d", piece)
	}
	fileArr[rank] = piece
	return nil
}

func (b *Board) GetPiece(file, rank uint32) (Piece, error) {
	if file >= uint32(len(b.Pieces)) {
		return 0, fmt.Errorf("board file of range: %d", file)
	}
	fileArr := &b.Pieces[file]
	if rank >= RanksPerFile[file] {
		return 0, fmt.Errorf("board rank of range: %d for file: %d", rank, file)
	}
	return fileArr[rank], nil
}

// SetPieceNot GetPieceNot set piece notation, get piece notation
func (b *Board) SetPieceNot(str string, p Piece) {
	hex := HexStr(str)
	b.Set(hex.File, hex.Rank, p)
}

func (b *Board) GetPieceNot(str string) Piece {
	hex := HexStr(str)
	return b.Get(hex.File, hex.Rank)
}

func (b *Board) FindKing(isWhite bool) (Hex, bool) {
	for _, hex := range OrdHexagons {
		piece := b.Get(hex.File, hex.Rank)
		if (piece == WhiteKing && isWhite) || (piece == BlackKing && !isWhite) {
			return hex, true
		}
	}
	return Hex{}, false
}

func (b *Board) InBounds(file, rank uint32) bool {
	return file < Files && rank < RanksPerFile[file]
}

func (b *Board) InBoundsHex(hex Hex) bool {
	return b.InBounds(hex.File, hex.Rank)
}

var ErrFenInvalidFiles = fmt.Errorf("FEN string should have %d files", Files)
var ErrFenMissingTurn = errors.New("FEN string is missing turn")

func ParseFen(fen string) (Board, error) {
	var board Board

	var file uint32
	var rank uint32
	var index int
	startCntIdx := -1

	for _, ch := range fen {
		if file >= Files {
			break
		}

		isSpace := unicode.IsSpace(ch)
		isDigit := unicode.IsDigit(ch)
		isLetter := unicode.IsLetter(ch)
		isSlash := ch == '/'

		if isSpace || isLetter || isSlash {
			if startCntIdx >= 0 {
				count, err := strconv.Atoi(fen[startCntIdx:index])
				if err != nil {
					return Board{}, err
				}
				rank += uint32(count)
			}
			startCntIdx = -1

			rps := RanksPerFile[file]

			if isSpace {
				break
			}
			if isLetter {
				if rank >= rps {
					return Board{}, fmt.Errorf("%s is ext of bounds", Hex{File: file, Rank: rank}.String())
				}
				piece, err := PieceFromRune(ch)
				if err != nil {
					return Board{}, fmt.Errorf("invalid FEN character: %c", ch)
				}
				board.Set(file, rank, piece)
				rank++
			} else {
				if rank != rps {
					return Board{}, fmt.Errorf("file '%c' requires %d pieces", file+'a', rps)
				}
				file++
				rank = 0
			}
		} else if isDigit {
			if startCntIdx < 0 {
				startCntIdx = index
			}
		} else {
			return Board{}, fmt.Errorf("invalid FEN character: %c", ch)
		}
		index++
	}

	if file != Files-1 {
		return Board{}, ErrFenInvalidFiles
	}

	for index < len(fen) && unicode.IsSpace(rune(fen[index])) {
		index++
	}
	if index >= len(fen) {
		return Board{}, ErrFenMissingTurn
	}

	ch := fen[index]
	switch ch {
	case 'w':
		board.IsWhiteTurn = true
	case 'b':
		board.IsWhiteTurn = false
	default:
		return Board{}, fmt.Errorf("invalid FEN turn: %c", ch)
	}

	return board, nil
}

func (b *Board) Fen() string {
	var sb strings.Builder

	for file := range Files {
		ranksCount := RanksPerFile[file]
		emptyCount := 0

		for rank := range ranksCount {
			piece := b.Get(file, rank)
			if piece == Empty {
				emptyCount++
				continue
			}

			if emptyCount > 0 {
				sb.WriteString(strconv.Itoa(emptyCount))
				emptyCount = 0
			}
			sb.WriteRune(piece.Rune())
		}

		if emptyCount > 0 {
			sb.WriteString(strconv.Itoa(emptyCount))
		}
		if file != Files-1 {
			sb.WriteRune('/')
		}
	}

	if b.IsWhiteTurn {
		sb.WriteString(" w")
	} else {
		sb.WriteString(" b")
	}

	return sb.String()
}

func (b *Board) String() string {
	// default: no moves highlighted
	return b.StringFunc(func(h Hex) rune { return 0 })
}

func (b *Board) StringFunc(isMove func(Hex) rune) string {
	var sb strings.Builder
	sb.WriteString("\n")

	for file := range Files {
		ranksCount := RanksPerFile[file]
		ranksDiff := MaxRanks - ranksCount

		fileChar := rune(file + 'a')
		sb.WriteRune(fileChar)
		sb.WriteString("   ")

		for range ranksDiff {
			sb.WriteString("  ")
		}

		for rank := range ranksCount {
			hex := Hex{File: file, Rank: rank}
			ch := isMove(hex)
			piece := b.Get(file, rank)
			if ch != 0 && piece == Empty {
				sb.WriteRune(ch)
			} else {
				sb.WriteRune(piece.Rune())
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
