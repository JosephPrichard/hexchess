package model

import (
	"crypto/rand"
	"hexchess-svc/chess"
	"log/slog"
	"math/big"
)

const GameIDChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789"
const GameIDPartitionChars = "abcdefghijklmnopqrstuvwxyz"

func randChar(str string) byte {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(str))))
	if err != nil {
		// just panic because this means static determined input is invalid (program)
		panic("failed to generate random number: " + err.Error())
	}
	return str[n.Int64()]
}

func GameIDPartitions() []string {
	partitions := make([]string, len(GameIDPartitionChars))
	for i, s := range GameIDPartitionChars {
		partitions[i] = string(s)
	}
	return partitions
}

type GameID string

const GameIDLength = 24

func NewGameID() GameID {
	gameID := make([]byte, GameIDLength)
	for i := range gameID {
		gameID[i] = randChar(GameIDChars)
	}

	gameID[len(gameID)-1] = randChar(GameIDPartitionChars)

	return GameID(gameID)
}

func (gameID GameID) String() string {
	return string(gameID)
}

func (gameID GameID) Partition() rune {
	if len(gameID) == 0 {
		slog.Error("returned an invalid partition for empty gameID")
		return 0
	}

	lastSymbol := rune(gameID[len(gameID)-1])

	return lastSymbol
}

type FinishedGame struct {
	GameID       GameID           `json:"id"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  int64            `json:"whitePlayer"`
	BlackPlayer  int64            `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayResult"`
	ReplayCause  ReplayCause      `json:"replayCause"`
}

type GameMetadataUpdt struct {
	GameID      GameID    `json:"id"`
	WhitePlayer int64     `json:"whitePlayer"`
	BlackPlayer int64     `json:"blackPlayer"`
	Mode        GameMode  `json:"mode"`
	FirstColor  GameColor `json:"firstColor"`
}

type UndoKind int

const (
	UndoCreate UndoKind = iota
	UndoAccept
	UndoReject
)
