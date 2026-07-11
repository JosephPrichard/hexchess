package model

import (
	"hexchess-svc/utils/enum"
	"slices"
	"time"
)

type GameMode int

const (
	ModeCorrespondence1 GameMode = iota
	ModeCorrespondence7
	ModeCorrespondence14
	ModeTimed1Plus0
	ModeTimed3Plus2
	ModeTimed15Plus10
)

var gameModeEntries = []enum.Entry[GameMode]{
	{Enum: ModeCorrespondence1, String: "CORRESPONDENCE_1"},
	{Enum: ModeCorrespondence7, String: "CORRESPONDENCE_7"},
	{Enum: ModeCorrespondence14, String: "CORRESPONDENCE_14"},
	{Enum: ModeTimed1Plus0, String: "TIMED_1+0"},
	{Enum: ModeTimed3Plus2, String: "TIMED_3+2"},
	{Enum: ModeTimed15Plus10, String: "TIMED_15+10"},
}

var GameModeEnums = enum.BuildReverseMap(gameModeEntries)

var RealTimeModes = []GameMode{
	ModeTimed1Plus0,
	ModeTimed3Plus2,
	ModeTimed15Plus10,
}

func (m GameMode) IsRealTime() bool {
	return slices.Contains(RealTimeModes, m)
}

func (m GameMode) TotalTime() time.Duration {
	switch m {
	case ModeTimed1Plus0:
		return 1 * time.Minute
	case ModeTimed3Plus2:
		return 3 * time.Minute
	case ModeTimed15Plus10:
		return 15 * time.Minute
	default:
		return 0
	}
}

func (m GameMode) TimeIncrement() time.Duration {
	switch m {
	case ModeTimed1Plus0:
		return 0
	case ModeTimed3Plus2:
		return 2 * time.Second
	case ModeTimed15Plus10:
		return 10 * time.Second
	default:
		return 0
	}
}

func (m GameMode) String() string { return enum.String(m, gameModeEntries) }

func (m GameMode) MarshalJSON() ([]byte, error) {
	return enum.Marshal(m, gameModeEntries)
}

func (m *GameMode) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, GameModeEnums, m)
}

func ExpectGameMode[S enum.StringLike](s S) GameMode {
	return enum.Expect(s, GameModeEnums)
}
