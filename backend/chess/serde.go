package chess

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/pb"
)

// Piece

func DeserializePieces(src []int32) []Piece {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Piece, 0, len(src))
	for _, p := range src {
		dst = append(dst, Piece(p))
	}
	return dst
}

func SerializePieces(pieces []Piece) []int32 {
	out := make([]int32, 0, len(pieces))
	for _, p := range pieces {
		out = append(out, int32(p))
	}
	return out
}

// Move

func DeserializeMove(pbPm *pb.Move) Move {
	if pbPm == nil {
		return Move{}
	}
	return Move{
		From:      Hex{File: uint32(pbPm.FromFile), Rank: uint32(pbPm.FromRank)},
		To:        Hex{File: uint32(pbPm.ToFile), Rank: uint32(pbPm.ToRank)},
		Promotion: Promotion(pbPm.Promotion),
	}
}

// PiecesMoves

func DeserializePiecesMoves(pbMoves []*pb.PieceMoves) []PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsArr := make([]PieceMoves, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moves := make([]Hex, 0, len(pbPm.Moves))
		for _, hInt := range pbPm.Moves {
			file := uint32(hInt & 0xFFFFFFFF)
			rank := uint32(hInt >> 32)
			moves = append(moves, Hex{File: file, Rank: rank})
		}
		pms := PieceMoves{
			Piece: Piece(pbPm.Piece),
			From:  Hex{File: uint32(pbPm.FromFile), Rank: uint32(pbPm.FromRank)},
			Moves: moves,
		}
		pmsArr = append(pmsArr, pms)
	}
	return pmsArr
}

func SerializePiecesMoves(moves []PieceMoves) []*pb.PieceMoves {
	if len(moves) == 0 {
		return nil
	}
	pbMoves := make([]*pb.PieceMoves, 0, len(moves))
	for _, pm := range moves {
		pbHexes := make([]int64, 0, len(pm.Moves))
		for _, h := range pm.Moves {
			hInt := int64(h.Rank)<<32 | int64(h.File)
			pbHexes = append(pbHexes, hInt)
		}
		pbMoves = append(pbMoves, &pb.PieceMoves{
			Piece:    int32(pm.Piece),
			FromFile: int32(pm.From.File),
			FromRank: int32(pm.From.Rank),
			Moves:    pbHexes,
		})
	}
	return pbMoves
}

// Board

var ErrNilBoard = errors.New("board must not be nil")

func DeserializeBoard(pbBoard *pb.ChessBoard) (Board, error) {
	if pbBoard == nil {
		return Board{}, ErrNilBoard
	}

	var serdeErrs []error

	board := Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for file, bFile := range pbBoard.File {
		for rank, piece := range bFile.Pieces {
			p := Piece(piece)
			if err := board.SetPiece(uint32(file), uint32(rank), p); err != nil {
				serdeErrs = append(serdeErrs, err)
				continue
			}
		}
	}
	return board, errors.Join(serdeErrs...)
}

func SerializeBoard(board *Board) *pb.ChessBoard {
	if board == nil {
		return nil
	}
	files := make([]*pb.BoardFile, 0, Files)
	for file := range Files {
		ranksCount := RanksPerFile[file]
		pieces := make([]uint32, 0, ranksCount)
		for rank := range ranksCount {
			piece, err := board.GetPiece(file, rank)
			if err != nil {
				// we panic here because it is programmer error if this code fails.
				panic(fmt.Errorf("get piece: %w", err))
			}
			pieces = append(pieces, uint32(piece))
		}
		files = append(files, &pb.BoardFile{Pieces: pieces})
	}
	return &pb.ChessBoard{File: files, IsWhiteTurn: board.IsWhiteTurn}
}

// HistMove

func DeserializeHistMove(pbHm *pb.HistMove) HistMove {
	if pbHm == nil {
		return HistMove{}
	}
	return HistMove{
		PieceMove: PieceMove{
			Piece: Piece(pbHm.Piece),
			From:  Hex{File: uint32(pbHm.FromFile), Rank: uint32(pbHm.FromRank)},
			To:    Hex{File: uint32(pbHm.ToFile), Rank: uint32(pbHm.ToRank)},
		},
		Notation:   pbHm.Notation,
		WhiteTimer: time.Duration(pbHm.WhiteTimerMs) * time.Millisecond,
		BlackTimer: time.Duration(pbHm.BlackTimerMs) * time.Millisecond,
	}
}

func DeserializeHistMoveList(pbMoves []*pb.HistMove) []HistMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moves := make([]HistMove, 0, len(pbMoves))
	for _, pbHm := range pbMoves {
		moves = append(moves, DeserializeHistMove(pbHm))
	}
	return moves
}

func SerializeHistMove(hm HistMove) *pb.HistMove {
	return &pb.HistMove{
		Piece:        int32(hm.Piece),
		FromFile:     int32(hm.From.File),
		FromRank:     int32(hm.From.Rank),
		ToFile:       int32(hm.To.File),
		ToRank:       int32(hm.To.Rank),
		Notation:     hm.Notation,
		WhiteTimerMs: hm.WhiteTimer.Milliseconds(),
		BlackTimerMs: hm.BlackTimer.Milliseconds(),
	}
}

func SerializeMoveList(moves []HistMove) []*pb.HistMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.HistMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, SerializeHistMove(pm))
	}
	return pbMoveList
}

// Game

var ErrNilGame = errors.New("board must not be nil")

func DeserializeGame(pbGame *pb.ChessGame) (g Game, err error) {
	if pbGame == nil {
		return g, ErrNilGame
	}
	board, err := DeserializeBoard(pbGame.Board)
	if err != nil {
		return g, fmt.Errorf("deserialize board %v: %w", pbGame.Board, err)
	}
	return Game{
		TakenWhitePieces: DeserializePieces(pbGame.TakenWhitePieces),
		TakenBlackPieces: DeserializePieces(pbGame.TakenBlackPieces),
		BlackMoves:       DeserializePiecesMoves(pbGame.BlackMoves),
		WhiteMoves:       DeserializePiecesMoves(pbGame.WhiteMoves),
		Moves:            DeserializeHistMoveList(pbGame.Moves),
		Board:            board,
	}, nil
}

func SerializeGame(game *Game) *pb.ChessGame {
	if game == nil {
		return nil
	}
	return &pb.ChessGame{
		TakenWhitePieces: SerializePieces(game.TakenWhitePieces),
		TakenBlackPieces: SerializePieces(game.TakenBlackPieces),
		BlackMoves:       SerializePiecesMoves(game.BlackMoves),
		WhiteMoves:       SerializePiecesMoves(game.WhiteMoves),
		Moves:            SerializeMoveList(game.Moves),
		Board:            SerializeBoard(&game.Board),
	}
}

// MoveHistory

func MarshalMoveHistory(initialBoard Board, moveSeq []HistMove) ([]byte, error) {
	game := Game{Board: initialBoard}
	game.InitPieceMoves()

	pbInitialGame := SerializeGame(&game)

	var pbMoveSteps []*pb.HistMove
	for _, m := range moveSeq {
		//game.NewMove(Move{From: m.From, To: m.To, Promotion: m.Promotion})
		//game.InitPieceMoves()

		pbMoveSteps = append(pbMoveSteps, SerializeHistMove(m))
	}

	return proto.Marshal(&pb.MoveHistory{InitialGame: pbInitialGame, Steps: pbMoveSteps})
}

func ExtractMoveHistoryLastBoard(data []byte) ([]byte, error) {
	var pbMoveHistory pb.MoveHistory
	if err := proto.Unmarshal(data, &pbMoveHistory); err != nil {
		return nil, err
	}

	game, err := DeserializeGame(pbMoveHistory.InitialGame)
	if err != nil {
		return nil, fmt.Errorf("deserialize initial game: %w", err)
	}
	steps := DeserializeHistMoveList(pbMoveHistory.Steps)

	for _, step := range steps {
		game.NewMove(Move{From: step.From, To: step.To, Promotion: step.Promotion})
		game.InitPieceMoves()
	}

	pbBoard := SerializeBoard(&game.Board)
	return proto.Marshal(pbBoard)
}
