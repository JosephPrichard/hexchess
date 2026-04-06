package svc

import (
	"hexchess-svc/util/enum"
)

type TournamentStatus int

const (
	TournamentLobby TournamentStatus = iota
	TournamentInProgress
	TournamentFinished
)

var tournamentStatusEntries = []enum.Entry[TournamentStatus]{
	{TournamentLobby, "LOBBY"},
	{TournamentInProgress, "IN_PROGRESS"},
	{TournamentFinished, "FINISHED"},
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
