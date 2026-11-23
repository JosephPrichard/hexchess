package gameid

import (
	"crypto/rand"
	"log/slog"
	"math/big"
)

const GameIDChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789"
const GameIDPartitionChars = "123456789"

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

type Identifier string

const GameIDLength = 24

func NewGameID() Identifier {
	gameID := make([]byte, GameIDLength)
	for i := range gameID {
		gameID[i] = randChar(GameIDChars)
	}

	gameID[len(gameID)-1] = randChar(GameIDPartitionChars)

	return Identifier(gameID)
}

func (gameID Identifier) String() string {
	return string(gameID)
}

func (gameID Identifier) Partition() rune {
	if len(gameID) == 0 {
		slog.Error("returned an invalid partition for empty gameID")
		return 0
	}

	lastSymbol := rune(gameID[len(gameID)-1])
	return lastSymbol
}
