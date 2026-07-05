package model

import (
	"crypto/rand"
	"hexchess-lib/optional"
	"hexchess-svc/chess"
	"math/big"
)

type GameID string

const GameIDSymbols = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func NewGameID() GameID {
	gameID := make([]byte, 8)
	for i := range gameID {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(GameIDSymbols))))
		if err != nil {
			panic("failed to generate random number: " + err.Error())
		}
		gameID[i] = GameIDSymbols[n.Int64()]
	}
	return GameID(gameID)
}

func (id GameID) String() string {
	return string(id)
}

func (id GameID) Partition() rune {
	if len(id) == 0 {
		return 0
	}
	return rune(id[len(id)-1])
}

func GameIDPartitions() []string {
	partitions := make([]string, 0, len(GameIDSymbols))
	for _, s := range GameIDSymbols {
		partitions = append(partitions, string(s))
	}
	return partitions
}

type FinishedGame struct {
	GameID       GameID           `json:"id"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  PlayerState      `json:"whitePlayer"`
	BlackPlayer  PlayerState      `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayResult"`
	ReplayCause  ReplayCause      `json:"replayCause"`
}

type GameMetadataUpdt struct {
	GameID      GameID                `json:"id"`
	WhitePlayer optional.Maybe[int64] `json:"whitePlayer"`
	BlackPlayer optional.Maybe[int64] `json:"blackPlayer"`
	Mode        GameMode              `json:"mode"`
	FirstColor  GameColor             `json:"firstColor"`
}
