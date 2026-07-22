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
	ReplayColorElos
}

type FullMatch struct {
	Ordering      int64             `json:"ordering"`
	GameID        GameID            `json:"gameID"`
	TournamentKey uuid.UUID         `json:"tournamentKey"`
	Round         int32             `json:"round"`
	CreatedOn     time.Time         `json:"createdOn"`
	WhiteID       int64             `json:"whiteId"`
	BlackID       int64             `json:"blackId"`
	Replay        *TournamentReplay `json:"replay"`
}

type MatchCreation struct {
	GameID   GameID   `json:"gameID"`
	GameMode GameMode `json:"gameMode"`
	WhiteID  int64    `json:"whiteId"`
	BlackID  int64    `json:"blackId"`
}

type MatchCreations struct {
	Creations []MatchCreation `json:"creations"`
}

type FullTournament struct {
	Participants []Participant `json:"participants"`
	Matches      []FullMatch   `json:"matches"`
	Tournament
}

type AdvanceTournamentEvent struct {
	EventID       uuid.UUID `json:"eventId"`
	TournamentKey uuid.UUID `json:"tournamentKey"`
}

type TournamentOutputKind string

const (
	TournamentParticipantKind TournamentOutputKind = "participant"
	TournamentCountdownKind   TournamentOutputKind = "countdown"
	TournamentStartKind       TournamentOutputKind = "start"
	TournamentMatchmakingKind TournamentOutputKind = "matchmaking"
	TournamentErrorKind       TournamentOutputKind = "error"
)

type TournamentOutputKey struct {
	TournamentKey string `json:"tournamentKey"`
}

type TournamentOutput struct {
	Key             string               `json:"tournamentKey"`
	Kind            TournamentOutputKind `json:"kind"`
	Matches         []FullMatch          `json:"matches"`
	Error           string               `json:"error"`
	LeaderboardUser LbdUser              `json:"leaderboardUser"`
}
