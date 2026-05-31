package model

import (
	"hexchess-svc/chess"
	"hexchess-svc/lib/enum"
)

type FinishedGame struct {
	GameID       string           `json:"id"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  PlayerState      `json:"whitePlayer"`
	BlackPlayer  PlayerState      `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayresult"`
	ReplayCause  ReplayCause      `json:"replaycause"`
}

type GameMetadataUpdt struct {
	GameID      string               `json:"id"`
	WhitePlayer enum.Optional[int64] `json:"whitePlayer"`
	BlackPlayer enum.Optional[int64] `json:"blackPlayer"`
	Mode        GameMode             `json:"mode"`
	FirstColor  GameColor            `json:"firstColor"`
}
