package game

import (
	"hexchess-svc/chess"
	"hexchess-svc/gamedata"
	gameid "hexchess-svc/gameid"
	"hexchess-svc/player"
	"hexchess-svc/replay"
	"hexchess-svc/utils/opt"
	"time"
)

type Chat struct {
	ID      string             `json:"id"`
	Player  player.PlayerState `json:"player"`
	Message string             `json:"message"`
	SentAt  time.Time          `json:"sentAt"`
}

type FinishGameEvent struct {
	GameID       gameid.Identifier   `json:"id"`
	InitialBoard chess.Board         `json:"initialBoard"`
	Moves        []chess.HistMove    `json:"moves"`
	WhiteID      int64               `json:"whiteID"`
	BlackID      int64               `json:"blackID"`
	ReplayMode   gamedata.GameMode   `json:"mode"`
	ReplayResult replay.ReplayResult `json:"replayResult"`
	ReplayCause  replay.ReplayCause  `json:"replayCause"`
	InsertedTime time.Time           `json:"insertedTime"`
}

type UpdtGameMetadataEvent struct {
	GameID      gameid.Identifier  `json:"id"`
	WhitePlayer opt.Option[int64]  `json:"whitePlayer"`
	BlackPlayer opt.Option[int64]  `json:"blackPlayer"`
	Mode        gamedata.GameMode  `json:"mode"`
	FirstColor  gamedata.GameColor `json:"firstColor"`
}

type UndoKind int

const (
	UndoCreate UndoKind = iota
	UndoAccept
	UndoReject
)

type ErrorGameOutput struct {
	GameID    gameid.Identifier `json:"gameId"`
	MessageID string            `json:"messageId"`
	Error     error             `json:"message"`
}

type InitGameOutput struct {
	GameID gameid.Identifier  `json:"gameId"`
	State  *ChessState        `json:"state"`
	Self   player.PlayerState `json:"self"`
}

type PlayersGameOutput struct {
	GameID      gameid.Identifier  `json:"gameId"`
	MessageID   string             `json:"messageId"`
	WhitePlayer player.PlayerState `json:"whitePlayer"`
	BlackPlayer player.PlayerState `json:"blackPlayer"`
}

type MoveGameOutput struct {
	GameID    gameid.Identifier `json:"gameId"`
	MessageID string            `json:"messageId"`
	Move      chess.HistMove    `json:"move"`
	State     *ChessState       `json:"game"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type ChatGameOutput struct {
	GameID    gameid.Identifier `json:"gameId"`
	MessageID string            `json:"messageId"`
	Chat      Chat              `json:"chat"`
}

type UndoGameOutput struct {
	GameID    gameid.Identifier `json:"gameId"`
	MessageID string            `json:"messageId"`
	Kind      string            `json:"kind"`
	UndoID    int64             `json:"undoId"`
	State     *ChessState       `json:"game"`
}

type ForfeitGameOutput struct {
	GameID    gameid.Identifier `json:"gameId"`
	MessageID string            `json:"messageId"`
	EndState  EndKind           `json:"endState"`
}

type ReplayGameOutput struct {
	GameID gameid.Identifier `json:"gameId"`
	Replay replay.FullReplay `json:"replay"`
}
