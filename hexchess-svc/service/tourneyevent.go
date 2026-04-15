package svc

import "errors"

type TournamentOutputKey string

const (
	ParticipantKey TournamentOutputKey = "participant"
	CountdownKey   TournamentOutputKey = "countdown"
	StartKey       TournamentOutputKey = "start"
	MatchmakingKey TournamentOutputKey = "matchmaking"
	ErrorKey       TournamentOutputKey = "error"
)

var ErrStartTournamentTaskQueue = errors.New("failed to handle start scheduled tournament event on task queue")

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
	Matches []Match `json:"matches"`
}

func (m TournamentOutput_Matchmaking) isTournamentOutput_Value() {}

type TournamentOutput_Error string

func (e TournamentOutput_Error) isTournamentOutput_Value() {}
