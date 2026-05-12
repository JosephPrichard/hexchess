package model

import "hexchess-svc/chess"

type FinishedGame struct {
	GameID       string           `json:"existingID"`
	Board        chess.Board      `json:"board"`
	Moves        []chess.HistMove `json:"moves"`
	WhitePlayer  PlayerState      `json:"whitePlayer"`
	BlackPlayer  PlayerState      `json:"blackPlayer"`
	ReplayMode   GameMode         `json:"mode"`
	ReplayResult ReplayResult     `json:"replayresult"`
	ReplayCause  ReplayCause      `json:"replaycause"`
}
