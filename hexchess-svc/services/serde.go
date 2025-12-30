package svc

import (
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"time"
)

func UnmarshalPlayer(b []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(b, &pbPlayer); err != nil {
		return PlayerState{}, err
	}
	player := PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest, Present: true}
	return player, nil
}

func MarshalPlayer(p PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
	return proto.Marshal(&pbPlayer)
}

func DeserializePlayer(pbPlayer *pb.PlayerState) PlayerState {
	var player PlayerState
	if pbPlayer != nil {
		player = PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest, Present: true}
	}
	return player
}

func UnmarshalChessState(b []byte) (ChessState, error) {
	var state ChessState

	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return state, err
	}
	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return state, fmt.Errorf("deserialize game: %w", err)
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.Game.Board)
	if err != nil {
		return state, fmt.Errorf("deserialize board %v: %w", pbChess.Game.Board, err)
	}

	color, err := ParseColor(pbChess.FirstColor)
	if err != nil {
		return state, err
	}
	mode, err := ParseGameMode(pbChess.Mode)
	if err != nil {
		return state, err
	}

	state = ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoState:    UndoState{UndoID: pbChess.UndoId},
		ChessMeta: ChessMeta{
			ID:          pbChess.Id,
			WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
			BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
			IsEnded:     pbChess.IsEnded,
			FirstColor:  color,
			Mode:        mode,
			Touch:       time.UnixMilli(pbChess.Touch),
		},
	}
	return state, nil
}

func SerializePlayer(p PlayerState) *pb.PlayerState {
	if p.Present {
		return &pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
	}
	return nil
}

func SerializeChessState(s *ChessState) *pb.ChessState {
	if s == nil {
		return nil
	}
	return &pb.ChessState{
		Id:           s.ID,
		Game:         chess.SerializeGame(&s.Game),
		WhitePlayer:  SerializePlayer(s.WhitePlayer),
		BlackPlayer:  SerializePlayer(s.BlackPlayer),
		IsEnded:      s.IsEnded,
		FirstColor:   s.FirstColor.String(),
		Mode:         s.Mode.String(),
		Touch:        s.Touch.UnixMilli(),
		InitialBoard: chess.SerializeBoard(&s.InitialBoard),
		UndoId:       s.UndoID,
	}
}

func UnmarshalChessMeta(b []byte) (ChessMeta, error) {
	var m ChessMeta

	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return m, fmt.Errorf("unmarshal chess state: %w", err)
	}

	color, err := ParseColor(pbChess.FirstColor)
	if err != nil {
		return m, err
	}
	mode, err := ParseGameMode(pbChess.Mode)
	if err != nil {
		return m, err
	}

	m = ChessMeta{
		ID:          pbChess.Id,
		WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
		IsEnded:     pbChess.IsEnded,
		FirstColor:  color,
		Mode:        mode,
	}
	return m, nil
}

func MarshalUserMsgJson(pbUm *pb.UserMsg) ([]byte, error) {
	if cm := pbUm.GetChallenge(); cm != nil {
		madeOn, err := time.Parse(time.RFC3339, cm.MadeOn)
		if err != nil {
			return nil, fmt.Errorf("parse challenge made on: %w", err)
		}
		return json.Marshal(ChallengeEntity{
			ChallengerID:      cm.ChallengerId,
			ChallengerName:    cm.ChallengerName,
			ChallengerCountry: cm.ChallengerCountry,
			ChallengerElo:     cm.ChallengerElo,
			ChallengeeID:      cm.ChallengeeId,
			ChallengeeName:    cm.ChallengeeName,
			ChallengeeCountry: cm.ChallengeeCountry,
			ChallengeeElo:     cm.ChallengeeElo,
			StartColor:        cm.StartColor,
			Mode:              cm.Mode,
			MadeOn:            madeOn,
		})
	}
	return nil, fmt.Errorf("unknown message type: %T", pbUm)
}

func SerializeChallengeMsg(ce ChallengeEntity) *pb.UserMsg {
	cm := &pb.UserMsg_Challenge{
		Challenge: &pb.ChallengeMsg{
			ChallengerId:      ce.ChallengerID,
			ChallengerName:    ce.ChallengerName,
			ChallengerCountry: ce.ChallengerCountry,
			ChallengerElo:     ce.ChallengerElo,
			ChallengeeId:      ce.ChallengeeID,
			ChallengeeName:    ce.ChallengeeName,
			ChallengeeCountry: ce.ChallengeeCountry,
			ChallengeeElo:     ce.ChallengeeElo,
			Mode:              ce.Mode,
			StartColor:        ce.StartColor,
			MadeOn:            ce.MadeOn.Format(time.RFC3339),
		},
	}
	return &pb.UserMsg{UserId: ce.ChallengeeID, Value: cm}
}
