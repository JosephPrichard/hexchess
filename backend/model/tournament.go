package model

import (
	"time"

	"github.com/google/uuid"
)

type Tournament struct {
	ID                 int64             `json:"id"`
	TournamentKey      uuid.UUID         `json:"tournamentKey"`
	Name               string            `json:"name"`
	Rounds             int32             `json:"rounds"`
	WinnerID           int64             `json:"winnerId"`
	MaxPlayerCount     int               `json:"maxPlayerCount"`
	CountdownStartedOn time.Time         `json:"countdownStartedOn"`
	CountdownStarted   bool              `json:"isCountdownStarted"`
	Countdown          string            `json:"countdown"`
	CreatedOn          time.Time         `json:"createdOn"`
	CreatedBy          int64             `json:"createdBy"`
	Ruleset            TournamentRuleset `json:"ruleset"`
	Status             TournamentStatus  `json:"status"`
	Mode               GameMode          `json:"mode"`
}

type Participant = LbdUser

type TournamentReplay struct {
	Replay
	RepayView
}

type Match struct {
	Ordering      int64             `json:"ordering"`
	GameID        string            `json:"gameID"`
	TournamentKey uuid.UUID         `json:"tournamentKey"`
	Round         int32             `json:"round"`
	CreatedOn     time.Time         `json:"createdOn"`
	WhiteID       int64             `json:"whiteId"`
	BlackID       int64             `json:"blackId"`
	Replay        *TournamentReplay `json:"replay"`
}

type FullTournament struct {
	Participants []Participant `json:"participants"`
	Matches      []Match       `json:"matches"`
	Tournament
}
