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
	pmsArr := make([]PieceMoves, 0, len(pbMoves))
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
		pmsArr = append(pmsArr, pms)
	}
	return pmsArr
}

var ErrNilBoard = errors.New("board must not be nil")

func MapBoard(pbBoard *pb.ChessBoard) (Board, error) {
	if pbBoard == nil {
		return Board{}, ErrNilBoard
	}
	board := Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for file, bFile := range pbBoard.File {
		for rank, piece := range bFile.Pieces {
			if err := board.SetPiece(file, rank, Piece(piece)); err != nil {
				return board, fmt.Errorf("failed to set piece: %w", err)
			}
		}
	}
	return board, nil
}

func MapHistMove(pbHm *pb.HistMove) HistMove {
	if pbHm == nil {
		return HistMove{}
	}
	return HistMove{
		PieceMove: PieceMove{
			Piece: Piece(pbHm.Piece),
			From:  Hex{File: int(pbHm.FromFile), Rank: int(pbHm.FromRank)},
			To:    Hex{File: int(pbHm.ToFile), Rank: int(pbHm.ToRank)},
		},
		CollFile: pbHm.CollFile,
		CollRank: pbHm.CollRank,
		IsCheck:  pbHm.IsCheck,
		IsTake:   pbHm.IsTake,
	}
}

func MapHistMoveList(pbMoves []*pb.HistMove) []HistMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moves := make([]HistMove, 0, len(pbMoves))
	for _, pbHm := range pbMoves {
		moves = append(moves, MapHistMove(pbHm))
	}
	return moves
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
		Moves:            MapHistMoveList(pbGame.Moves),
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
	files := make([]*pb.BoardFile, 0, Files)
	for file := range Files {
		ranksCount := RanksPerFile[file]
		pieces := make([]uint32, 0, ranksCount)
		for rank := range ranksCount {
			piece, err := board.GetPiece(file, rank)
			if err != nil {
				return nil, err
			}
			pieces = append(pieces, uint32(piece))
		}
		files = append(files, &pb.BoardFile{Pieces: pieces})
	}
	return &pb.ChessBoard{File: files, IsWhiteTurn: board.IsWhiteTurn}, nil
}

func MapPbHistMove(hm HistMove) *pb.HistMove {
	return &pb.HistMove{
		Piece:    int32(hm.Piece),
		FromFile: int32(hm.From.File),
		FromRank: int32(hm.From.Rank),
		ToFile:   int32(hm.To.File),
		ToRank:   int32(hm.To.Rank),
		CollFile: hm.CollFile,
		CollRank: hm.CollRank,
		IsCheck:  hm.IsCheck,
		IsTake:   hm.IsTake,
	}
}

func MapPbMoveList(moves []HistMove) []*pb.HistMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.HistMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, MapPbHistMove(pm))
	}
	return pbMoveList
}

func MapPbGame(game Game) (*pb.ChessGame, error) {
	pbBoard, err := MapPbBoard(game.Board)
	if err != nil {
		return nil, fmt.Errorf("failed to map board: %w", err)
	}
	pbGame := &pb.ChessGame{
		TakenWhitePieces: MapPbPieces(game.TakenWhitePieces),
		TakenBlackPieces: MapPbPieces(game.TakenBlackPieces),
		BlackMoves:       MapPbPiecesMoves(game.BlackMoves),
		WhiteMoves:       MapPbPiecesMoves(game.WhiteMoves),
		Moves:            MapPbMoveList(game.Moves),
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

func MapPbMoveReplay(pbMoveHist *pb.MoveHistory) *pb.MoveReplay {
	pbFmtSteps := make([]*pb.NotMoveStep, 0, len(pbMoveHist.Steps))
	for _, pbStep := range pbMoveHist.Steps {
		hm := MapHistMove(pbStep.Move)
		pbFmtSteps = append(pbFmtSteps, &pb.NotMoveStep{
			Game:    pbStep.Game,
			Pm:      MapPbPieceMove(hm.PieceMove),
			NotMove: hm.String(),
		})
	}
	return &pb.MoveReplay{
		InitialGame: pbMoveHist.InitialGame,
		Steps:       pbFmtSteps,
	}
}

func MarshalMoveHistory(initialBoard Board, moveSeq []HistMove) ([]byte, error) {
	game := Game{Board: initialBoard}
	game.InitPieceMoves()

	pbInitialGame, err := MapPbGame(game)
	if err != nil {
		return nil, fmt.Errorf("failed to map initial game: %w", err)
	}

	var pbMoveSteps []*pb.MoveStep
	for _, m := range moveSeq {
		game.MakeMove(m.From, m.To)
		game.InitPieceMoves()

		pbGame, err := MapPbGame(game)
		if err != nil {
			return nil, fmt.Errorf("failed to map game for move %s: %w", m.String(), err)
		}

		pbMoveSteps = append(pbMoveSteps, &pb.MoveStep{
			Game: pbGame,
			Move: MapPbHistMove(m),
		})
	}

	return proto.Marshal(&pb.MoveHistory{InitialGame: pbInitialGame, Steps: pbMoveSteps})
}
