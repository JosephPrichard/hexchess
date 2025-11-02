package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"strconv"
	"time"
)

func UnmarshalPlayer(b []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(b, &pbPlayer); err != nil {
		return PlayerState{}, err
	}
	player := PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest}
	return player, nil
}

func MarshalPlayer(p *PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
	return proto.Marshal(&pbPlayer)
}

func MapPlayer(pbPlayer *pb.PlayerState) *PlayerState {
	var player *PlayerState
	if pbPlayer != nil {
		player = &PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest}
	}
	return player
}

func MapPieces(src []int32) []chess.Piece {
	if len(src) == 0 {
		return nil
	}
	dst := make([]chess.Piece, 0, len(src))
	for _, p := range src {
		dst = append(dst, chess.Piece(p))
	}
	return dst
}

func MapPieceMove(pbPm *pb.PieceMove) chess.PieceMove {
	if pbPm == nil {
		return chess.PieceMove{}
	}
	return chess.PieceMove{
		Piece: chess.Piece(pbPm.Piece),
		From:  chess.Hex{File: int(pbPm.FromFile), Rank: int(pbPm.FromRank)},
		To:    chess.Hex{File: int(pbPm.ToFile), Rank: int(pbPm.ToRank)},
	}
}

func MapPiecesMoves(pbMoves []*pb.PieceMoves) []chess.PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsList := make([]chess.PieceMoves, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moves := make([]chess.Hex, 0, len(pbPm.Moves))
		for _, hInt := range pbPm.Moves {
			file := int(hInt & 0xFFFFFFFF)
			rank := int(hInt >> 32)
			moves = append(moves, chess.Hex{File: file, Rank: rank})
		}
		pms := chess.PieceMoves{
			Piece: chess.Piece(pbPm.Piece),
			From:  chess.Hex{File: int(pbPm.FromFile), Rank: int(pbPm.FromRank)},
			Moves: moves,
		}
		pmsList = append(pmsList, pms)
	}
	return pmsList
}

func MapBoard(pbBoard *pb.ChessBoard) (chess.Board, error) {
	board := chess.Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for f, file := range pbBoard.File {
		if f >= chess.Files {
			return board, fmt.Errorf("board file is out of bounds: %d", f)
		}
		for r, piece := range file.Pieces {
			if err := board.SetPiece(f, r, chess.Piece(piece)); err != nil {
				return board, err
			}
		}
	}
	return board, nil
}

func MapMoveList(pbMoves []*pb.PieceMove) []chess.PieceMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moveList := make([]chess.PieceMove, 0, len(pbMoves))
	for _, pbPm := range pbMoves {
		moveList = append(moveList, MapPieceMove(pbPm))
	}
	return moveList
}

var ErrNilGame = errors.New("game and board must not be nil")

func UnmarshalChess(b []byte) (ChessState, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessState{}, fmt.Errorf("failed to unmarhsal chess state: %w", err)
	}
	if pbChess.Game == nil || pbChess.Game.Board == nil {
		return ChessState{}, ErrNilGame
	}

	board, err := MapBoard(pbChess.Game.Board)
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to map board: %w", err)
	}

	game := chess.Game{
		TakenWhitePieces: MapPieces(pbChess.Game.TakenWhitePieces),
		TakenBlackPieces: MapPieces(pbChess.Game.TakenBlackPieces),
		BlackMoves:       MapPiecesMoves(pbChess.Game.BlackMoves),
		WhiteMoves:       MapPiecesMoves(pbChess.Game.WhiteMoves),
		Board:            board,
	}

	state := ChessState{
		Game:     game,
		MoveList: MapMoveList(pbChess.MoveList),
		ChessMeta: ChessMeta{
			ID:          pbChess.Id,
			WhitePlayer: MapPlayer(pbChess.WhitePlayer),
			BlackPlayer: MapPlayer(pbChess.BlackPlayer),
			IsEnded:     pbChess.IsEnded,
			FirstColor:  ColorSelect(pbChess.FirstColor),
			TimeControl: TimeControl(pbChess.TimeControl),
			Touch:       time.UnixMilli(pbChess.Touch),
		},
	}
	return state, nil
}

func MapPbPlayer(p *PlayerState) *pb.PlayerState {
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

func MapPbPieces(pieces []chess.Piece) []int32 {
	out := make([]int32, 0, len(pieces))
	for _, p := range pieces {
		out = append(out, int32(p))
	}
	return out
}

func MapPbPieceMove(pm chess.PieceMove) *pb.PieceMove {
	return &pb.PieceMove{
		Piece:    int32(pm.Piece),
		FromFile: int32(pm.From.File),
		FromRank: int32(pm.From.Rank),
		ToFile:   int32(pm.From.File),
		ToRank:   int32(pm.To.Rank),
	}
}

func MapPbPiecesMoves(moves []chess.PieceMoves) []*pb.PieceMoves {
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

func MapPbBoard(board chess.Board) (*pb.ChessBoard, error) {
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

func MapPbMoveList(moves []chess.PieceMove) []*pb.PieceMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.PieceMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, MapPbPieceMove(pm))
	}
	return pbMoveList
}

func MapPbGame(game chess.Game) (*pb.ChessGame, error) {
	pbBoard, err := MapPbBoard(game.Board)
	if err != nil {
		return nil, fmt.Errorf("failed to map pb board: %w", err)
	}
	pbGame := &pb.ChessGame{
		TakenWhitePieces: MapPbPieces(game.TakenWhitePieces),
		TakenBlackPieces: MapPbPieces(game.TakenBlackPieces),
		BlackMoves:       MapPbPiecesMoves(game.BlackMoves),
		WhiteMoves:       MapPbPiecesMoves(game.WhiteMoves),
		Board:            pbBoard,
	}
	return pbGame, nil
}

func MapPbChessState(s ChessState) (*pb.ChessState, error) {
	pbGame, err := MapPbGame(s.Game)
	if err != nil {
		return nil, fmt.Errorf("failed to map pb game: %w", err)
	}

	var pbMoveList []*pb.PieceMove
	if s.MoveList != nil {
		pbMoveList = make([]*pb.PieceMove, 0, len(s.MoveList))
		for _, pm := range s.MoveList {
			pbMoveList = append(pbMoveList, MapPbPieceMove(pm))
		}
	}

	pbState := &pb.ChessState{
		Id:          s.ID,
		Game:        pbGame,
		MoveList:    MapPbMoveList(s.MoveList),
		WhitePlayer: MapPbPlayer(s.WhitePlayer),
		BlackPlayer: MapPbPlayer(s.BlackPlayer),
		IsEnded:     s.IsEnded,
		FirstColor:  uint32(s.FirstColor),
		TimeControl: uint32(s.TimeControl),
		Touch:       s.Touch.UnixMilli(),
	}
	return pbState, nil
}

func MarshalChessState(s ChessState) ([]byte, error) {
	pbState, err := MapPbChessState(s)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pbState)
}

func UnmarshalChessMeta(b []byte) (ChessMeta, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessMeta{}, fmt.Errorf("failed to unmarhsal chess state: %w", err)
	}
	cv := ChessMeta{
		ID:          pbChess.Id,
		WhitePlayer: MapPlayer(pbChess.WhitePlayer),
		BlackPlayer: MapPlayer(pbChess.BlackPlayer),
		IsEnded:     pbChess.IsEnded,
		FirstColor:  ColorSelect(pbChess.FirstColor),
		TimeControl: TimeControl(pbChess.TimeControl),
	}
	return cv, nil
}

func MarshalUserMsgJson(pbUm *pb.UserMsg) ([]byte, error) {
	if c := pbUm.GetChallenge(); c != nil {
		return json.Marshal(ChallengeEntity{
			ChallengerID:      c.ChallengerId,
			ChallengerName:    c.ChallengerName,
			ChallengerCountry: c.ChallengerCountry,
			ChallengerElo:     c.ChallengerElo,
			ChallengeeID:      c.ChallengeeId,
			ChallengeeName:    c.ChallengeeName,
			ChallengeeCountry: c.ChallengeeCountry,
			ChallengeeElo:     c.ChallengeeElo,
			TimeControl:       TimeControl(c.TimeControl),
			StartColor:        ColorSelect(c.StartColor),
			MadeOn:            time.UnixMilli(c.MadeOn),
		})
	}
	return nil, fmt.Errorf("unknown message type: %T", pbUm)
}

func MapPbChallengeMsg(id int64, c ChallengeEntity) pb.UserMsg {
	return pb.UserMsg{
		UserId: strconv.Itoa(int(id)),
		Value: &pb.UserMsg_Challenge{Challenge: &pb.ChallengeMsg{
			ChallengerId:      c.ChallengerID,
			ChallengerName:    c.ChallengerName,
			ChallengerCountry: c.ChallengerCountry,
			ChallengerElo:     c.ChallengerElo,
			ChallengeeId:      c.ChallengeeID,
			ChallengeeName:    c.ChallengeeName,
			ChallengeeCountry: c.ChallengeeCountry,
			ChallengeeElo:     c.ChallengeeElo,
			TimeControl:       uint32(c.TimeControl),
			StartColor:        uint32(c.StartColor),
			MadeOn:            c.MadeOn.UnixMilli(),
		}},
	}
}
