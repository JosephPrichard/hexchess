package player

import (
	"hexchess-svc/pb"
)

// Player

func UnmarshalPlayer(bytes []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := pbPlayer.UnmarshalVT(bytes); err != nil {
		return PlayerState{}, err
	}
	return DeserializePlayer(&pbPlayer), nil
}

func MarshalPlayer(player PlayerState) ([]byte, error) {
	return SerializePlayer(player).MarshalVT()
}

func DeserializePlayer(pbPlayer *pb.PlayerState) PlayerState {
	if pbPlayer != nil {
		return PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}
	} else {
		return PlayerState{}
	}
}

func SerializePlayer(player PlayerState) *pb.PlayerState {
	var pbPlayer *pb.PlayerState
	if player.Present {
		pbPlayer = &pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country}
	}
	return pbPlayer
}
