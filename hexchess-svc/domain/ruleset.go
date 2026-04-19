package domain

import (
	"hexchess-svc/util/enum"
)

type TournamentRuleset int

const (
	TournamentKnockout TournamentRuleset = iota
	TournamentRoundRobin
	TournamentSwiss
)

var tournamentRulesetEntries = []enum.Entry[TournamentRuleset]{
	{Enum: TournamentKnockout, String: "KNOCKOUT"},
	{Enum: TournamentRoundRobin, String: "ROUND_ROBIN"},
	{Enum: TournamentSwiss, String: "SWISS"},
}

var TournamentRulesetEnums = enum.BuildReverseMap(tournamentRulesetEntries)

func (t TournamentRuleset) String() string {
	return enum.String(t, tournamentRulesetEntries)
}

func (t TournamentRuleset) MarshalJSON() ([]byte, error) {
	return enum.Marshal(t, tournamentRulesetEntries)
}

func (t *TournamentRuleset) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, TournamentRulesetEnums, t)
}
