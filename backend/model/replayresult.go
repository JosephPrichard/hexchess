package model

import "hexchess-svc/lib/enum"

type ReplayResult int

const (
	WhiteWin ReplayResult = iota
	BlackWin
	Draw
)

var replayResultEntries = []enum.Entry[ReplayResult]{
	{Enum: WhiteWin, String: "WHITE_WINS"},
	{Enum: BlackWin, String: "BLACK_WINS"},
	{Enum: Draw, String: "DRAW"},
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
