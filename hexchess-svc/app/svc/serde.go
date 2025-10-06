package svc

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
)

func PlayerFromPb(pbPlayer *pb.PlayerState) PlayerState {
	if pbPlayer == nil {
		return PlayerState{}
	}
	return PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Elo: pbPlayer.Elo, IsGuest: pbPlayer.IsGuest}
}

func OptPlayerFromPb(pbPlayer *pb.PlayerState) *PlayerState {
	var player *PlayerState
	if pbPlayer != nil {
		t := PlayerFromPb(pbPlayer)
		player = &t
	}
	return player
}

func PlayerDeserialize(b []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(b, &pbPlayer); err != nil {
		return PlayerState{}, err
	}
	return PlayerFromPb(&pbPlayer), nil
}

func (p *PlayerState) Serialize() ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, Elo: p.Elo, IsGuest: p.IsGuest}
	return proto.Marshal(&pbPlayer)
}

func ChessDeserialize(b []byte) (ChessState, error) {
	var pbuf pb.ChessState
	if err := proto.Unmarshal(b, &pbuf); err != nil {
		return ChessState{}, err
	}
	state := ChessState{
		ID:          pbuf.Id,
		WhitePlayer: OptPlayerFromPb(pbuf.WhitePlayer),
		BlackPlayer: OptPlayerFromPb(pbuf.BlackPlayer),
		IsEnded:     pbuf.IsEnded,
		FirstColor:  ColorSelect(pbuf.FirstColor),
		TimeControl: TimeControl(pbuf.TimeControl),
	}
	return state, nil
}

func (c *ChessState) Serialize() ([]byte, error) {
	return nil, nil
}

func ChessViewDeserialize(str string) (ChessView, error) {
	var pbuf pb.ChessState
	if err := proto.Unmarshal([]byte(str), &pbuf); err != nil {
		return ChessView{}, err
	}
	cv := ChessView{
		ID:          pbuf.Id,
		WhitePlayer: OptPlayerFromPb(pbuf.WhitePlayer),
		BlackPlayer: OptPlayerFromPb(pbuf.BlackPlayer),
		IsEnded:     pbuf.IsEnded,
		FirstColor:  ColorSelect(pbuf.FirstColor),
		TimeControl: TimeControl(pbuf.TimeControl),
	}
	return cv, nil
}
