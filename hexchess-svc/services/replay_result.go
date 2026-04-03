package svc

import "hexchess-svc/internal/enum"

type ReplayResult int

const (
	WhiteWin ReplayResult = iota
	BlackWin
	Draw
)

var replayResultEntries = []enum.Entry[ReplayResult]{
	{WhiteWin, "WHITE_WINS"},
	{BlackWin, "BLACK_WINS"},
	{Draw, "DRAW"},
}

var ReplayResultEnums = enum.BuildReverseMap(replayResultEntries)

func (r ReplayResult) String() string { return enum.String(r, replayResultEntries) }

func (r ReplayResult) MarshalJSON() ([]byte, error) {
	return enum.Marshal(r, replayResultEntries)
}

func (r *ReplayResult) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, ReplayResultEnums, r)
}

func ExpectReplayResult[S enum.StringLike](s S) ReplayResult {
	return enum.Expect(s, ReplayResultEnums)
}
