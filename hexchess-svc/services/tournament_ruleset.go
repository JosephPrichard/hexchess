package svc

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
	{TournamentKnockout, "KNOCKOUT"},
	{TournamentRoundRobin, "ROUND_ROBIN"},
	{TournamentSwiss, "SWISS"},
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
