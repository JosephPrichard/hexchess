package chess

import (
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
)

func MapPieces(src []int32) []Piece {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Piece, 0, len(src))
	for _, p := range src {
		dst = append(dst, Piece(p))
	}
	return dst
}

func MapPieceMove(pbPm *pb.PieceMove) PieceMove {
	if pbPm == nil {
		return PieceMove{}
	}
	return PieceMove{
		Piece: Piece(pbPm.Piece),
		From:  Hex{File: int(pbPm.FromFile), Rank: int(pbPm.FromRank)},
		To:    Hex{File: int(pbPm.ToFile), Rank: int(pbPm.ToRank)},
	}
}

func MapPiecesMoves(pbMoves []*pb.PieceMoves) []PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsList := make([]PieceMoves, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moves := make([]Hex, 0, len(pbPm.Moves))
		for _, hInt := range pbPm.Moves {
			file := int(hInt & 0xFFFFFFFF)
			rank := int(hInt >> 32)
			moves = append(moves, Hex{File: file, Rank: rank})
		}
		pms := PieceMoves{
			Piece: Piece(pbPm.Piece),
			From:  Hex{File: int(pbPm.FromFile), Rank: int(pbPm.FromRank)},
			Moves: moves,
		}
		pmsList = append(pmsList, pms)
	}
	return pmsList
}

var ErrNilBoard = errors.New("board must not be nil")

func MapBoard(pbBoard *pb.ChessBoard) (Board, error) {
	if pbBoard == nil {
		return Board{}, ErrNilBoard
	}
	board := Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for f, file := range pbBoard.File {
		if f >= Files {
			return board, fmt.Errorf("board file is out of bounds: %d", f)
		}
		for r, piece := range file.Pieces {
			if err := board.SetPiece(f, r, Piece(piece)); err != nil {
				return board, fmt.Errorf("failed to set piece: %w", err)
			}
		}
	}
	return board, nil
}

func MapMoveList(pbMoves []*pb.PieceMove) []PieceMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moveList := make([]PieceMove, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moveList = append(moveList, MapPieceMove(pbPm))
	}
	return moveList
}

func MapGame(pbGame *pb.ChessGame) (Game, error) {
	board, err := MapBoard(pbGame.Board)
	if err != nil {
		return Game{}, fmt.Errorf("failed to map board: %w", err)
	}
	return Game{
		TakenWhitePieces: MapPieces(pbGame.TakenWhitePieces),
		TakenBlackPieces: MapPieces(pbGame.TakenBlackPieces),
		BlackMoves:       MapPiecesMoves(pbGame.BlackMoves),
		WhiteMoves:       MapPiecesMoves(pbGame.WhiteMoves),
		MoveList:         MapMoveList(pbGame.MoveList),
		Board:            board,
	}, nil
}

func MapPbPieceMove(pm PieceMove) *pb.PieceMove {
	return &pb.PieceMove{
		Piece:    int32(pm.Piece),
		FromFile: int32(pm.From.File),
		FromRank: int32(pm.From.Rank),
		ToFile:   int32(pm.To.File),
		ToRank:   int32(pm.To.Rank),
	}
}

func MapPbPiecesMoves(moves []PieceMoves) []*pb.PieceMoves {
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

func MapPbBoard(board Board) (*pb.ChessBoard, error) {
	pbBoard := &pb.ChessBoard{
		File:        make([]*pb.BoardFile, 0, Files),
		IsWhiteTurn: board.IsWhiteTurn,
	}
	for file := 0; file < Files; file++ {
		ranksCount := RanksPerFile[file]
		pbFile := &pb.BoardFile{
			Pieces: make([]uint32, 0, ranksCount),
		}
		for rank := 0; rank < ranksCount; rank++ {
			piece, err := board.GetPiece(file, rank)
			if err != nil {
				return nil, fmt.Errorf("failed to get piece: %w", err)
			}
			pbFile.Pieces = append(pbFile.Pieces, uint32(piece))
		}
		pbBoard.File = append(pbBoard.File, pbFile)
	}
	return pbBoard, nil
}

func MapPbMoveList(moves []PieceMove) []*pb.PieceMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.PieceMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, MapPbPieceMove(pm))
	}
	return pbMoveList
}

func MapPbGame(game Game) (*pb.ChessGame, error) {
	pbBoard, err := MapPbBoard(game.Board)
	if err != nil {
		return nil, fmt.Errorf("failed to map pb board: %w", err)
	}
	pbGame := &pb.ChessGame{
		TakenWhitePieces: MapPbPieces(game.TakenWhitePieces),
		TakenBlackPieces: MapPbPieces(game.TakenBlackPieces),
		BlackMoves:       MapPbPiecesMoves(game.BlackMoves),
		WhiteMoves:       MapPbPiecesMoves(game.WhiteMoves),
		MoveList:         MapPbMoveList(game.MoveList),
		Board:            pbBoard,
	}
	return pbGame, nil
}

func MapPbPieces(pieces []Piece) []int32 {
	out := make([]int32, 0, len(pieces))
	for _, p := range pieces {
		out = append(out, int32(p))
	}
	return out
}

func MarshalMoveHistory(initialBoard Board, moveList []PieceMove) ([]byte, error) {
	pbInitialBoard, err := MapPbBoard(initialBoard)
	if err != nil {
		return nil, fmt.Errorf("failed to map initial board: %w", err)
	}

	game := Game{Board: initialBoard}
	var pbMoveSteps []*pb.MoveStep
	for _, m := range moveList {
		game.InitPieceMoves()
		game.MakeMove(m.From, m.To)

		pbGame, err := MapPbGame(game)
		if err != nil {
			return nil, fmt.Errorf("failed to map pb game: %w", err)
		}

		pbMoveSteps = append(pbMoveSteps, &pb.MoveStep{
			Game: pbGame,
			Move: MapPbPieceMove(m),
		})
	}

	return proto.Marshal(&pb.MoveHistory{InitialBoard: pbInitialBoard, MoveSteps: pbMoveSteps})
}
