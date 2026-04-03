package svc

import "hexchess-svc/internal/enum"

type ReplayCause int

const (
	Checkmate ReplayCause = iota
	Forfeit
	Stalemate
)

var replayCauseEntries = []enum.Entry[ReplayCause]{
	{Checkmate, "CHECKMATE"},
	{Forfeit, "FORFEIT"},
	{Stalemate, "STALEMATE"},
}

var ReplayCauseEnums = enum.BuildReverseMap(replayCauseEntries)

func (c ReplayCause) String() string { return enum.String(c, replayCauseEntries) }

func (c ReplayCause) MarshalJSON() ([]byte, error) {
	return enum.Marshal(c, replayCauseEntries)
}

func (c *ReplayCause) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, ReplayCauseEnums, c)
}

func ExpectReplayCause[S enum.StringLike](s S) ReplayCause {
	return enum.Expect(s, ReplayCauseEnums)
}
