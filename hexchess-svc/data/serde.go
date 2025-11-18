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

func UnmarshalChess(b []byte) (ChessState, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessState{}, fmt.Errorf("failed to unmarhsal chess state: %w", err)
	}
	if pbChess.Game == nil || pbChess.Game.Board == nil {
		return ChessState{}, errors.New("game and board cannot be nil")
	}

	game, err := chess.MapGame(pbChess.Game)
	if err != nil {
		return ChessState{}, fmt.Errorf("failed to map game: %w", err)
	}

	state := ChessState{
		Game: game,
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

func MapPbChessState(s ChessState) (*pb.ChessState, error) {
	pbGame, err := chess.MapPbGame(s.Game)
	if err != nil {
		return nil, fmt.Errorf("failed to map pb game: %w", err)
	}

	pbState := &pb.ChessState{
		Id:          s.ID,
		Game:        pbGame,
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
		return ChessMeta{}, fmt.Errorf("failed to unmarshal chess state: %w", err)
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
