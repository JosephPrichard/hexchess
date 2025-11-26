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

var ErrNilChess = errors.New("chess board and game cannot be nil")

func UnmarshalChess(b []byte) (ChessState, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return ChessState{}, err
	}
	if pbChess.Game == nil || pbChess.Game.Board == nil {
		return ChessState{}, ErrNilChess
	}

	game, err := chess.MapGame(pbChess.Game)
	if err != nil {
		return ChessState{}, err
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
	return &pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
}

func MapPbChessState(s ChessState) (*pb.ChessState, error) {
	pbGame, err := chess.MapPbGame(s.Game)
	if err != nil {
		return nil, err
	}
	pbState := &pb.ChessState{
		Id:          s.ID,
		Game:        pbGame,
		WhitePlayer: MapPbPlayer(s.WhitePlayer),
		BlackPlayer: MapPbPlayer(s.BlackPlayer),
		IsEnded:     s.IsEnded,
		FirstColor:  string(s.FirstColor),
		TimeControl: string(s.TimeControl),
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
	cm := ChessMeta{
		ID:          pbChess.Id,
		WhitePlayer: MapPlayer(pbChess.WhitePlayer),
		BlackPlayer: MapPlayer(pbChess.BlackPlayer),
		IsEnded:     pbChess.IsEnded,
		FirstColor:  ColorSelect(pbChess.FirstColor),
		TimeControl: TimeControl(pbChess.TimeControl),
	}
	return cm, nil
}

func MarshalUserMsgJson(pbUm *pb.UserMsg) ([]byte, error) {
	if cm := pbUm.GetChallenge(); cm != nil {
		madeOn, err := time.Parse(time.RFC3339, cm.MadeOn)
		if err != nil {
			return nil, fmt.Errorf("failed to parse challenge made on: %w", err)
		}
		ce := ChallengeEntity{
			ChallengerID:      cm.ChallengerId,
			ChallengerName:    cm.ChallengerName,
			ChallengerCountry: cm.ChallengerCountry,
			ChallengerElo:     cm.ChallengerElo,
			ChallengeeID:      cm.ChallengeeId,
			ChallengeeName:    cm.ChallengeeName,
			ChallengeeCountry: cm.ChallengeeCountry,
			ChallengeeElo:     cm.ChallengeeElo,
			TimeControl:       TimeControl(cm.TimeControl),
			StartColor:        ColorSelect(cm.StartColor),
			MadeOn:            madeOn,
		}
		return json.Marshal(ce)
	}
	return nil, fmt.Errorf("unknown message type: %T", pbUm)
}

func MapPbChallengeMsg(id int64, ce ChallengeEntity) pb.UserMsg {
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
			TimeControl:       string(ce.TimeControl),
			StartColor:        string(ce.StartColor),
			MadeOn:            ce.MadeOn.Format(time.RFC3339),
		},
	}
	return pb.UserMsg{
		UserId: strconv.Itoa(int(id)),
		Value:  cm,
	}
}
