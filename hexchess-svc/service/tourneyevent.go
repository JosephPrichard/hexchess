package svc

type TournamentOutputKey string

const (
	ParticipantKey TournamentOutputKey = "participant"
	CountdownKey   TournamentOutputKey = "countdown"
	StartKey       TournamentOutputKey = "start"
	MatchmakingKey TournamentOutputKey = "matchmaking"
	ErrorKey       TournamentOutputKey = "error"
)

type TournamentOutput struct {
	Key   TournamentOutputKey      `json:"key"`
	Value isTournamentOutput_Value `json:"value"`
}

type isTournamentOutput_Value interface {
	isTournamentOutput_Value()
}

type TournamentOutput_Participant LbdUserDTO

func (p TournamentOutput_Participant) isTournamentOutput_Value() {}

type TournamentOutput_Countdown struct{}

func (c TournamentOutput_Countdown) isTournamentOutput_Value() {}

type TournamentOutput_Start struct{}

func (s TournamentOutput_Start) isTournamentOutput_Value() {}

type TournamentOutput_Matchmaking struct {
	Matches []MatchDTO `json:"matches"`
}

func (m TournamentOutput_Matchmaking) isTournamentOutput_Value() {}

type TournamentOutput_Error string

func (e TournamentOutput_Error) isTournamentOutput_Value() {}
