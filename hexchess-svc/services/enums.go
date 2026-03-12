package svc

import (
	"fmt"
	"time"

	"golang.org/x/exp/slices"
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

var RealTimeModes = []GameMode{ModeTimed1Plus0, ModeTimed3Plus2, ModeTimed15Plus10}

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

func (m GameMode) TimeIncr() time.Duration {
	switch m {
	case ModeTimed1Plus0:
		return 0 * time.Second
	case ModeTimed3Plus2:
		return 2 * time.Second
	case ModeTimed15Plus10:
		return 10 * time.Second
	default:
		return 0
	}
}

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

func ParseReplayCause(s string) (ReplayCause, error) {
	c, ok := ReplayCauseMap[s]
	if !ok {
		return 0, &OneOfError[ReplayCause]{Expected: ReplayCauseMap, Actual: s}
	}
	return c, nil
}

func ExpectReplayCause(s string) ReplayCause {
	c, err := ParseReplayCause(s)
	if err != nil {
		panic(err)
	}
	return c
}

func ParseReplayResult(s string) (ReplayResult, error) {
	r, ok := ReplayResultMap[s]
	if !ok {
		return 0, &OneOfError[ReplayResult]{Expected: ReplayResultMap, Actual: s}
	}
	return r, nil
}

func ExpectReplayResult(s string) ReplayResult {
	r, err := ParseReplayResult(s)
	if err != nil {
		panic(err)
	}
	return r
}

func ParseGameMode(s string) (GameMode, error) {
	m, ok := GameModeMap[s]
	if !ok {
		return 0, &OneOfError[GameMode]{Expected: GameModeMap, Actual: s}
	}
	return m, nil
}

func ExpectGameMode(s string) GameMode {
	m, err := ParseGameMode(s)
	if err != nil {
		panic(err)
	}
	return m
}

func ParseColor(s string) (Color, error) {
	c, ok := ColorMap[s]
	if !ok {
		return 0, &OneOfError[Color]{Expected: ColorMap, Actual: s}
	}
	return c, nil
}

func ExpectColor(s string) Color {
	m, err := ParseColor(s)
	if err != nil {
		panic(err)
	}
	return m
}

type EnumParser struct {
	err error
}

func (p *EnumParser) Err() error {
	return p.err
}

func (p *EnumParser) ReplayCause(s string) ReplayCause {
	if p.err != nil {
		return 0
	}
	v, err := ParseReplayCause(s)
	p.err = err
	return v
}

func (p *EnumParser) ReplayResult(s string) ReplayResult {
	if p.err != nil {
		return 0
	}
	v, err := ParseReplayResult(s)
	p.err = err
	return v
}

func (p *EnumParser) GameMode(s string) GameMode {
	if p.err != nil {
		return 0
	}
	v, err := ParseGameMode(s)
	p.err = err
	return v
}

func (p *EnumParser) Color(s string) Color {
	if p.err != nil {
		return 0
	}
	v, err := ParseColor(s)
	p.err = err
	return v
}

type OneOfError[T any] struct {
	Expected map[string]T
	Actual   string
}

func (err *OneOfError[T]) Error() string {
	return fmt.Sprintf("expected one of %v, got %v", err.Expected, err.Actual)
}
