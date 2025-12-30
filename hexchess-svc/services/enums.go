package svc

import (
	"fmt"
)

type ReplayResult int

const (
	_ ReplayResult = iota
	WhiteWin
	BlackWin
	Draw
)

func (r ReplayResult) String() string {
	switch r {
	case WhiteWin:
		return "WHITE_WINS"
	case BlackWin:
		return "BLACK_WINS"
	case Draw:
		return "DRAW"
	default:
		return "UNKNOWN"
	}
}

var ReplayResultMap = map[string]ReplayResult{
	"WHITE_WINS": WhiteWin,
	"BLACK_WINS": BlackWin,
	"DRAW":       Draw,
}

type ReplayCause int

const (
	_ ReplayCause = iota
	Checkmate
	Forfeit
	Stalemate
)

func (c ReplayCause) String() string {
	switch c {
	case Checkmate:
		return "CHECKMATE"
	case Forfeit:
		return "FORFEIT"
	case Stalemate:
		return "STALEMATE"
	default:
		return "UNKNOWN"
	}
}

var ReplayCauseMap = map[string]ReplayCause{
	"CHECKMATE": Checkmate,
	"FORFEIT":   Forfeit,
	"STALEMATE": Stalemate,
}

type GameMode int

const (
	_ GameMode = iota
	ModeTimed1Plus0
	ModeTimed3Plus2
	ModeTimed15Plus10
	ModeCorrespondence1
	ModeCorrespondence7
	ModeCorrespondence14
)

func (m GameMode) String() string {
	switch m {
	case ModeTimed1Plus0:
		return "TIMED_1+0"
	case ModeTimed3Plus2:
		return "TIMED_3+2"
	case ModeTimed15Plus10:
		return "TIMED_15+10"
	case ModeCorrespondence1:
		return "CORRESPONDENCE_1"
	case ModeCorrespondence7:
		return "CORRESPONDENCE_7"
	case ModeCorrespondence14:
		return "CORRESPONDENCE_14"
	default:
		return "UNKNOWN"
	}
}

var GameModeMap = map[string]GameMode{
	"TIMED_1+0":         ModeTimed1Plus0,
	"TIMED_3+2":         ModeTimed3Plus2,
	"TIMED_15+10":       ModeTimed15Plus10,
	"CORRESPONDENCE_1":  ModeCorrespondence1,
	"CORRESPONDENCE_7":  ModeCorrespondence7,
	"CORRESPONDENCE_14": ModeCorrespondence14,
}

type Color int

const (
	Random Color = iota
	White
	Black
)

func (c Color) String() string {
	switch c {
	case White:
		return "WHITE"
	case Black:
		return "BLACK"
	case Random:
		return "RANDOM"
	default:
		return "UNKNOWN"
	}
}

var ColorMap = map[string]Color{
	"WHITE":  White,
	"BLACK":  Black,
	"RANDOM": Random,
}

func ParseReplayCause[StringLike ~string](s StringLike) (ReplayCause, error) {
	c, ok := ReplayCauseMap[string(s)]
	if !ok {
		return 0, &OneOfError[ReplayCause]{Expected: ReplayCauseMap, Actual: string(s)}
	}
	return c, nil
}

func ExpectReplayCause[StringLike ~string](s StringLike) ReplayCause {
	c, err := ParseReplayCause(s)
	if err != nil {
		panic(err)
	}
	return c
}

func ParseReplayResult[StringLike ~string](s StringLike) (ReplayResult, error) {
	r, ok := ReplayResultMap[string(s)]
	if !ok {
		return 0, &OneOfError[ReplayResult]{Expected: ReplayResultMap, Actual: string(s)}
	}
	return r, nil
}

func ExpectReplayResult[StringLike ~string](s StringLike) ReplayResult {
	r, err := ParseReplayResult(s)
	if err != nil {
		panic(err)
	}
	return r
}

func ParseGameMode[StringLike ~string](s StringLike) (GameMode, error) {
	m, ok := GameModeMap[string(s)]
	if !ok {
		return 0, &OneOfError[GameMode]{Expected: GameModeMap, Actual: string(s)}
	}
	return m, nil
}

func ExpectGameMode[StringLike ~string](s StringLike) GameMode {
	m, err := ParseGameMode(s)
	if err != nil {
		panic(err)
	}
	return m
}

func ParseColor[StringLike ~string](s StringLike) (Color, error) {
	c, ok := ColorMap[string(s)]
	if !ok {
		return 0, &OneOfError[Color]{Expected: ColorMap, Actual: string(s)}
	}
	return c, nil
}

func ExpectColor[StringLike ~string](s StringLike) Color {
	m, err := ParseColor(s)
	if err != nil {
		panic(err)
	}
	return m
}

type OneOfError[T any] struct {
	Expected map[string]T
	Actual   string
}

func (err *OneOfError[T]) Error() string {
	return fmt.Sprintf("expected one of %v, got %v", err.Expected, err.Actual)
}
