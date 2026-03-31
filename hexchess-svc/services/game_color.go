package svc

import "hexchess-svc/internal/enum"

type GameColor int

const (
	Random GameColor = iota
	White
	Black
)

var colorEntries = []enum.Entry[GameColor]{
	{Random, "RANDOM"},
	{White, "WHITE"},
	{Black, "BLACK"},
}

var GameColorMembers = enum.BuildReverseMap(colorEntries)

func (c GameColor) String() string { return enum.String(c, colorEntries) }

func (c GameColor) MarshalJSON() ([]byte, error) {
	return enum.Marshal(c, colorEntries)
}

func (c *GameColor) UnmarshalJSON(d []byte) error {
	return enum.Unmarshal(d, GameColorMembers, c)
}

func ExpectColor[S enum.StringLike](s S) GameColor {
	return enum.Expect(s, GameColorMembers)
}
