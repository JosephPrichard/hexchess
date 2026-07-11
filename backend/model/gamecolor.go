package model

import "hexchess-svc/utils/enum"

type GameColor int

const (
	Random GameColor = iota
	White
	Black
)

var colorEntries = []enum.Entry[GameColor]{
	{Enum: Random, String: "RANDOM"},
	{Enum: White, String: "WHITE"},
	{Enum: Black, String: "BLACK"},
}

var GameColorEnums = enum.BuildReverseMap(colorEntries)

func (c GameColor) String() string { return enum.String(c, colorEntries) }

func (c GameColor) MarshalJSON() ([]byte, error) {
	return enum.Marshal(c, colorEntries)
}

func (c *GameColor) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, GameColorEnums, c)
}

func ExpectColor[S enum.StringLike](s S) GameColor {
	return enum.Expect(s, GameColorEnums)
}
