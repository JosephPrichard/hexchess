package model

import (
	"crypto/rand"
	"hexchess-svc/chess"
	"hexchess-svc/utils/opt"
	"log/slog"
	"math/big"
	"time"
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

type FinishGameEvent struct {
	GameID       GameID           `json:"id"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  int64            `json:"whitePlayer"`
	BlackPlayer  int64            `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayResult"`
	ReplayCause  ReplayCause      `json:"replayCause"`
	InsertedTime time.Time        `json:"insertedTime"`
}

type UpdtGameMetadataEvent struct {
	GameID      GameID            `json:"id"`
	WhitePlayer opt.Option[int64] `json:"whitePlayer"`
	BlackPlayer opt.Option[int64] `json:"blackPlayer"`
	Mode        GameMode          `json:"mode"`
	FirstColor  GameColor         `json:"firstColor"`
}

type UndoKind int

const (
	UndoCreate UndoKind = iota
	UndoAccept
	UndoReject
)

type ErrorGameOutput struct {
	GameID    GameID `json:"gameId"`
	MessageID string `json:"messageId"`
	Error     error  `json:"message"`
}

type InitGameOutput struct {
	GameID GameID      `json:"gameId"`
	State  *ChessState `json:"state"`
	Self   PlayerState `json:"self"`
}

type PlayersGameOutput struct {
	GameID      GameID      `json:"gameId"`
	MessageID   string      `json:"messageId"`
	WhitePlayer PlayerState `json:"whitePlayer"`
	BlackPlayer PlayerState `json:"blackPlayer"`
}

type MoveGameOutput struct {
	GameID    GameID         `json:"gameId"`
	MessageID string         `json:"messageId"`
	Move      chess.HistMove `json:"move"`
	State     *ChessState    `json:"game"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type ChatGameOutput struct {
	GameID    GameID `json:"gameId"`
	MessageID string `json:"messageId"`
	Chat      Chat   `json:"chat"`
}

type UndoGameOutput struct {
	GameID    GameID      `json:"gameId"`
	MessageID string      `json:"messageId"`
	Kind      string      `json:"kind"`
	UndoID    int64       `json:"undoId"`
	State     *ChessState `json:"game"`
}

type ForfeitGameOutput struct {
	GameID    GameID  `json:"gameId"`
	MessageID string  `json:"messageId"`
	EndState  EndKind `json:"endState"`
}

type ReplayGameOutput struct {
	GameID GameID     `json:"gameId"`
	Replay FullReplay `json:"replay"`
}
