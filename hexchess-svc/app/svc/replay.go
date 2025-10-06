package svc

import "time"

type ReplayRow struct {
	ID           int64
	WhiteId      int64
	BlackId      int64
	WhiteName    string
	BlackName    string
	WhiteCountry string
	BlackCountry string
	Result       ReplayResult
	Cause        ReplayCause
	WinElo       float32
	LoseElo      float32
	WhiteElo     float32
	BlackElo     float32
	PlayedOn     time.Time
}

type ReplayResult int

const (
	WhiteWin ReplayResult = iota
	BlackWin
	Draw
)

type ReplayCause int

const (
	Checkmate ReplayCause = iota
	Forfeit
)
