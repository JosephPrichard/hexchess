package svc

import (
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/app/chess"
	"hexchess-svc/pb"
	"time"
)

func PlayerDeserialize(b []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(b, &pbPlayer); err != nil {
		return PlayerState{}, fmt.Errorf("failed to unmarhsal player: %w", err)
	}
	player := PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest}
	return player, nil
}

func (p *PlayerState) Serialize() ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
	return proto.Marshal(&pbPlayer)
}

func mapPlayer(pbPlayer *pb.PlayerState) *PlayerState {
	var player *PlayerState
	if pbPlayer != nil {
		player = &PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest}
	}
	return player
}

func mapPieces(src []int32) []chess.Piece {
	if len(src) == 0 {
		return nil
	}
	dst := make([]chess.Piece, 0, len(src))
	for _, p := range src {
		dst = append(dst, chess.Piece(p))
	}
	return dst
}

func mapPieceMove(pbPm *pb.PieceMove) chess.PieceMove {
	if pbPm == nil {
		return chess.PieceMove{}
	}
	return chess.PieceMove{
		Piece: chess.Piece(pbPm.Piece),
		From:  chess.Hex{File: int(pbPm.FromFile), Rank: int(pbPm.FromRank)},
		To:    chess.Hex{File: int(pbPm.ToFile), Rank: int(pbPm.ToRank)},
	}
}

func mapPiecesMoves(pbMoves []*pb.PieceMoves) []chess.PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsList := make([]chess.PieceMoves, 0, len(pbMoves))
	for _, pm := range pbMoves {
		moves := make([]chess.Hex, 0, len(pm.Moves))
		for _, hInt := range pm.Moves {
			file := int(hInt & 0xFFFFFFFF)
			rank := int(hInt >> 32)
			moves = append(moves, chess.Hex{File: file, Rank: rank})
		}
		pms := chess.PieceMoves{
			From:  chess.Hex{File: int(pm.FromFile), Rank: int(pm.FromRank)},
			Moves: moves,
		}
		pmsList = append(pmsList, pms)
	}
	return pmsList
}

func mapBoard(pbBoard *pb.ChessBoard) (chess.Board, error) {
	board := chess.MakeBoard(pbBoard.IsWhiteTurn)
	for f, file := range pbBoard.File {
		if f >= chess.Files {
			return board, fmt.Errorf("board file is out of bounds: %d", f)
		}
		for r, piece := range file.Pieces {
			if err := board.SafeSetPiece(f, r, chess.Piece(piece)); err != nil {
				return board, err
			}
		}
	}
	return board, nil
}

func mapMoveList(pbMoves []*pb.PieceMove) []chess.PieceMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moveList := make([]chess.PieceMove, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moveList = append(moveList, mapPieceMove(pbPm))
	}
	return moveList
}

var ErrNilGame = errors.New("game and board must not be nil")

func ChessDeserialize(b []byte) (ChessState, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessState{}, fmt.Errorf("failed to unmarhsal chess state: %w", err)
	}
	if pbChess.Game == nil || pbChess.Game.Board == nil {
		return ChessState{}, ErrNilGame
	}

	board, err := mapBoard(pbChess.Game.Board)
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to map board: %w", err)
	}

	game := chess.Game{
		TakenWhitePieces: mapPieces(pbChess.Game.TakenWhitePieces),
		TakenBlackPieces: mapPieces(pbChess.Game.TakenBlackPieces),
		BlackMoves:       mapPiecesMoves(pbChess.Game.BlackMoves),
		WhiteMoves:       mapPiecesMoves(pbChess.Game.WhiteMoves),
		Board:            board,
	}

	state := ChessState{
		ID:          pbChess.Id,
		Game:        game,
		MoveList:    mapMoveList(pbChess.MoveList),
		WhitePlayer: mapPlayer(pbChess.WhitePlayer),
		BlackPlayer: mapPlayer(pbChess.BlackPlayer),
		IsEnded:     pbChess.IsEnded,
		FirstColor:  ColorSelect(pbChess.FirstColor),
		TimeControl: TimeControl(pbChess.TimeControl),
		Touch:       time.UnixMilli(pbChess.Touch),
	}
	return state, nil
}

func mapPbPlayer(p *PlayerState) *pb.PlayerState {
	if p == nil {
		return nil
	}
	return &pb.PlayerState{
		Id:      p.ID,
		Name:    p.Name,
		Country: p.Country,
		Elo:     p.Elo,
		IsGuest: p.IsGuest,
	}
}

func mapPbPieces(pieces []chess.Piece) []int32 {
	out := make([]int32, 0, len(pieces))
	for _, p := range pieces {
		out = append(out, int32(p))
	}
	return out
}

func mapPbPieceMove(pm chess.PieceMove) *pb.PieceMove {
	return &pb.PieceMove{
		Piece:    int32(pm.Piece),
		FromFile: int32(pm.From.File),
		FromRank: int32(pm.From.Rank),
		ToFile:   int32(pm.From.File),
		ToRank:   int32(pm.To.Rank),
	}
}

func mapPbPiecesMoves(moves []chess.PieceMoves) []*pb.PieceMoves {
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
		pbMoves = append(pbMoves, &pb.PieceMoves{FromFile: int32(pm.From.File), FromRank: int32(pm.From.Rank), Moves: pbHexes})
	}
	return pbMoves
}

func mapPbBoard(board chess.Board) (*pb.ChessBoard, error) {
	pbBoard := &pb.ChessBoard{
		File:        make([]*pb.BoardFile, 0, chess.Files),
		IsWhiteTurn: board.IsWhiteTurn,
	}
	for file := 0; file < chess.Files; file++ {
		ranksCount := chess.RanksPerFile[file]
		pbFile := &pb.BoardFile{
			Pieces: make([]uint32, 0, ranksCount),
		}
		for rank := 0; rank < ranksCount; rank++ {
			piece, err := board.SafeGetPiece(file, rank)
			if err != nil {
				return nil, fmt.Errorf("failed to get piece: %w", err)
			}
			pbFile.Pieces = append(pbFile.Pieces, uint32(piece))
		}
		pbBoard.File = append(pbBoard.File, pbFile)
	}
	return pbBoard, nil
}

func mapPbMoveList(moves []chess.PieceMove) []*pb.PieceMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.PieceMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, mapPbPieceMove(pm))
	}
	return pbMoveList
}

func (s *ChessState) Serialize() ([]byte, error) {
	pbBoard, err := mapPbBoard(s.Game.Board)
	if err != nil {
		return nil, fmt.Errorf("failed to map pb board: %w", err)
	}

	pbGame := &pb.ChessGame{
		TakenWhitePieces: mapPbPieces(s.Game.TakenWhitePieces),
		TakenBlackPieces: mapPbPieces(s.Game.TakenBlackPieces),
		BlackMoves:       mapPbPiecesMoves(s.Game.BlackMoves),
		WhiteMoves:       mapPbPiecesMoves(s.Game.WhiteMoves),
		Board:            pbBoard,
	}

	var pbMoveList []*pb.PieceMove
	if s.MoveList != nil {
		pbMoveList = make([]*pb.PieceMove, 0, len(s.MoveList))
		for _, pm := range s.MoveList {
			pbMoveList = append(pbMoveList, mapPbPieceMove(pm))
		}
	}

	pbState := &pb.ChessState{
		Id:          s.ID,
		Game:        pbGame,
		MoveList:    mapPbMoveList(s.MoveList),
		WhitePlayer: mapPbPlayer(s.WhitePlayer),
		BlackPlayer: mapPbPlayer(s.BlackPlayer),
		IsEnded:     s.IsEnded,
		FirstColor:  uint32(s.FirstColor),
		TimeControl: uint32(s.TimeControl),
		Touch:       s.Touch.UnixMilli(),
	}
	return proto.Marshal(pbState)
}

func ChessViewDeserialize(b []byte) (ChessView, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessView{}, fmt.Errorf("failed to unmarhsal chess state: %w", err)
	}
	cv := ChessView{
		ID:          pbChess.Id,
		WhitePlayer: mapPlayer(pbChess.WhitePlayer),
		BlackPlayer: mapPlayer(pbChess.BlackPlayer),
		IsEnded:     pbChess.IsEnded,
		FirstColor:  ColorSelect(pbChess.FirstColor),
		TimeControl: TimeControl(pbChess.TimeControl),
	}
	return cv, nil
}
