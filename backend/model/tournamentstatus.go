package model

import (
	"hexchess-svc/utils/enum"
)

type TournamentStatus int

const (
	TournamentLobby TournamentStatus = iota
	TournamentScheduled
	TournamentInProgress
	TournamentFinished
	TournamentCancelled
)

var tournamentStatusEntries = []enum.Entry[TournamentStatus]{
	{Enum: TournamentLobby, String: "LOBBY"},
	{Enum: TournamentScheduled, String: "SCHEDULED"},
	{Enum: TournamentInProgress, String: "IN_PROGRESS"},
	{Enum: TournamentFinished, String: "FINISHED"},
	{Enum: TournamentCancelled, String: "CANCELLED"},
}

var TournamentStatusEnums = enum.BuildReverseMap(tournamentStatusEntries)

func (t TournamentStatus) String() string {
	return enum.String(t, tournamentStatusEntries)
}

func (t TournamentStatus) MarshalJSON() ([]byte, error) {
	return enum.Marshal(t, tournamentStatusEntries)
}

func (t *TournamentStatus) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, TournamentStatusEnums, t)
}
