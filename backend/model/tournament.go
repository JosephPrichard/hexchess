package model

import (
	"errors"
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
	GameID   GameID
	GameMode GameMode
	WhiteID  int64
	BlackID  int64
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

type TournamentOutputKey string

const (
	ParticipantKey TournamentOutputKey = "participant"
	CountdownKey   TournamentOutputKey = "countdown"
	StartKey       TournamentOutputKey = "start"
	MatchmakingKey TournamentOutputKey = "matchmaking"
	ErrorKey       TournamentOutputKey = "error"
)

var ErrAdvanceTournamentCode = errors.New("ERR_ADVANCE_TOURNAMENT")

type TournamentOutput struct {
	Key   TournamentOutputKey      `json:"key"`
	Value isTournamentOutput_Value `json:"value"`
}

type isTournamentOutput_Value interface {
	isTournamentOutput_Value()
}

type TournamentOutput_Participant LbdUser

func (p TournamentOutput_Participant) isTournamentOutput_Value() {}

type TournamentOutput_Countdown struct{}

func (c TournamentOutput_Countdown) isTournamentOutput_Value() {}

type TournamentOutput_Start struct{}

func (s TournamentOutput_Start) isTournamentOutput_Value() {}

type TournamentOutput_Matchmaking struct {
	Matches []FullMatch `json:"matches"`
}

func (m TournamentOutput_Matchmaking) isTournamentOutput_Value() {}

type TournamentOutput_Error string

func (e TournamentOutput_Error) isTournamentOutput_Value() {}
