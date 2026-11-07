package chess

import (
	"fmt"
	"hexchess-svc/pb"
	"time"
)

// Piece

func DeserializePieces(pbPieces []uint32) []Piece {
	if len(pbPieces) == 0 {
		return nil
	}
	pieces := make([]Piece, 0, len(pbPieces))
	for _, p := range pbPieces {
		pieces = append(pieces, Piece(p))
	}
	return pieces
}

func SerializePieces(pieces []Piece) []uint32 {
	pbPieces := make([]uint32, 0, len(pieces))
	for _, p := range pieces {
		pbPieces = append(pbPieces, uint32(p))
	}
	return pbPieces
}

// Move

func DeserializeMove(pbMove *pb.Move) Move {
	if pbMove == nil {
		return Move{}
	}
	return Move{
		From: Hex{
			File: pbMove.FromFile,
			Rank: pbMove.FromRank,
		},
		To: Hex{
			File: pbMove.ToFile,
			Rank: pbMove.ToRank,
		},
		Promotion: Promotion(pbMove.Promotion),
	}
}

// PiecesMoves

func DeserializePiecesMoves(pbMoves []*pb.PieceMoves) []PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsArr := make([]PieceMoves, 0, len(pbMoves))
	for _, pbMove := range pbMoves {
		if pbMove == nil {
			continue
		}
		moves := make([]Hex, 0, len(pbMove.Moves))
		for _, hex := range pbMove.Moves {
			if hex == nil {
				continue
			}
			moves = append(moves, Hex{File: hex.File, Rank: hex.Rank})
		}
		pms := PieceMoves{
			Piece: Piece(pbMove.Piece),
			From:  Hex{File: pbMove.FromFile, Rank: pbMove.FromRank},
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
		pbHexes := make([]*pb.Hex, 0, len(pm.Moves))
		for _, h := range pm.Moves {
			pbHexes = append(pbHexes, &pb.Hex{File: h.File, Rank: h.Rank})
		}
		pbMoves = append(pbMoves, &pb.PieceMoves{
			Piece:    uint32(pm.Piece),
			FromFile: pm.From.File,
			FromRank: pm.From.Rank,
			Moves:    pbHexes,
		})
	}
	return pbMoves
}

// Board

func DeserializeBoard(pbBoard *pb.ChessBoard) (Board, error) {
	if pbBoard == nil {
		return Board{}, nil
	}
	board := Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for file, bFile := range pbBoard.File {
		for rank, piece := range bFile.Pieces {
			p := Piece(piece)
			if err := board.SetPiece(uint32(file), uint32(rank), p); err != nil {
				return Board{}, err
			}
		}
	}
	return board, nil
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
				panic(fmt.Errorf("get piece: %w", err))
			}
			pieces = append(pieces, uint32(piece))
		}
		files = append(files, &pb.BoardFile{Pieces: pieces})
	}
	return &pb.ChessBoard{File: files, IsWhiteTurn: board.IsWhiteTurn}
}

// HistMove

func DeserializeHistMove(pbHistMove *pb.HistMove) HistMove {
	if pbHistMove == nil {
		return HistMove{}
	}
	return HistMove{
		Piece:      Piece(pbHistMove.Piece),
		From:       Hex{File: pbHistMove.FromFile, Rank: pbHistMove.FromRank},
		To:         Hex{File: pbHistMove.ToFile, Rank: pbHistMove.ToRank},
		Notation:   pbHistMove.Notation,
		WhiteTimer: Timer(pbHistMove.WhiteTimerMs) * Timer(time.Millisecond),
		BlackTimer: Timer(pbHistMove.BlackTimerMs) * Timer(time.Millisecond),
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

func SerializeHistMove(histMove HistMove) *pb.HistMove {
	return &pb.HistMove{
		Piece:        uint32(histMove.Piece),
		FromFile:     histMove.From.File,
		FromRank:     histMove.From.Rank,
		ToFile:       histMove.To.File,
		ToRank:       histMove.To.Rank,
		Notation:     histMove.Notation,
		WhiteTimerMs: histMove.WhiteTimer.Duration().Milliseconds(),
		BlackTimerMs: histMove.BlackTimer.Duration().Milliseconds(),
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

func DeserializeGame(pbGame *pb.ChessGame) (g Game, err error) {
	if pbGame == nil {
		return Game{}, err
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

func MarshalMoveHistory(moveHist MoveHistory) ([]byte, error) {
	game := Game{Board: moveHist.InitialBoard}
	game.InitPieceMoves()

	pbInitialGame := SerializeGame(&game)

	var pbMoveSteps []*pb.HistMove
	for _, m := range moveHist.MoveSeq {
		pbMoveSteps = append(pbMoveSteps, SerializeHistMove(m))
	}

	pbMoveHist := &pb.MoveHistory{InitialGame: pbInitialGame, Steps: pbMoveSteps}
	return pbMoveHist.MarshalVT()
}
