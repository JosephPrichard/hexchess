package model

import (
	"time"
)

type Replay struct {
	ID          int64        `json:"id"`
	WhiteID     int64        `json:"whiteId"`
	BlackID     int64        `json:"blackId"`
	Mode        GameMode     `json:"mode"`
	Result      ReplayResult `json:"result"`
	Cause       ReplayCause  `json:"cause"`
	WinEloDiff  float64      `json:"winEloDiff"`
	LoseEloDiff float64      `json:"loseEloDiff"`
	PlayedOn    time.Time    `json:"playedOn"`
	Rating      float64      `json:"rating"`
	TurnCount   int          `json:"turnCount"`
}

type ReplayUsers struct {
	WhiteName    string  `json:"whiteName"`
	BlackName    string  `json:"blackName"`
	WhiteCountry string  `json:"whiteCountry"`
	BlackCountry string  `json:"blackCountry"`
	WhiteElo     float64 `json:"whiteElo"`
	BlackElo     float64 `json:"blackElo"`
}

type ReplayColorElos struct {
	WhiteEloDiff float64 `json:"whiteEloDiff"`
	BlackEloDiff float64 `json:"blackEloDiff"`
}

type ReplayPreview struct {
	PreviewBoard []byte `json:"previewBoard"`
}

type FullReplay struct {
	Replay
	ReplayUsers
	ReplayColorElos
}

func NewReplayView(input Replay) (output ReplayColorElos) {
	switch input.Result {
	case WhiteWin:
		output.WhiteEloDiff, output.BlackEloDiff = input.WinEloDiff, input.LoseEloDiff
	case BlackWin:
		output.WhiteEloDiff, output.BlackEloDiff = input.LoseEloDiff, input.WinEloDiff
	default:
	}
	return
}
