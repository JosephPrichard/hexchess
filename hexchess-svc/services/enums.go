package svc

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

type stringLike interface {
	~string
}

type enumEntry[T ~int] struct {
	val  T
	name string
}

func buildReverseMap[T ~int](entries []enumEntry[T]) map[string]T {
	m := make(map[string]T, len(entries))
	for _, e := range entries {
		m[e.name] = e.val
	}
	return m
}

func enumString[T ~int](val T, entries []enumEntry[T]) string {
	for _, e := range entries {
		if e.val == val {
			return e.name
		}
	}
	return "UNKNOWN"
}

func marshalEnum[T ~int](val T, entries []enumEntry[T]) ([]byte, error) {
	for _, e := range entries {
		if e.val == val {
			return json.Marshal(e.name)
		}
	}
	return nil, fmt.Errorf("unknown enum value: %d", val)
}

func unmarshalEnum[T ~int](data []byte, entries []enumEntry[T], out *T) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, e := range entries {
		if e.name == s {
			*out = e.val
			return nil
		}
	}
	return fmt.Errorf("unknown enum value: %q", s)
}

func parseEnum[T ~int, S stringLike](s S, m map[string]T) (T, error) {
	v, ok := m[string(s)]
	if !ok {
		return 0, OneOfError[T]{Expected: m, Actual: string(s)}
	}
	return v, nil
}

func expectEnum[T ~int, S stringLike](s S, m map[string]T) T {
	v, err := parseEnum(s, m)
	if err != nil {
		panic(err.Error())
	}
	return v
}

type OneOfError[T any] struct {
	Expected map[string]T
	Actual   string
}

func (err OneOfError[T]) Error() string {
	return fmt.Sprintf("expected one of %v, got %v", err.Expected, err.Actual)
}

type ReplayResult int

const (
	_ ReplayResult = iota
	WhiteWin
	BlackWin
	Draw
)

var replayResultEntries = []enumEntry[ReplayResult]{
	{WhiteWin, "WHITE_WINS"},
	{BlackWin, "BLACK_WINS"},
	{Draw, "DRAW"},
}

var ReplayResultMap = buildReverseMap(replayResultEntries)

func (r ReplayResult) String() string                { return enumString(r, replayResultEntries) }
func (r ReplayResult) MarshalJSON() ([]byte, error)  { return marshalEnum(r, replayResultEntries) }
func (r *ReplayResult) UnmarshalJSON(d []byte) error { return unmarshalEnum(d, replayResultEntries, r) }

func ParseReplayResult[S stringLike](s S) (ReplayResult, error) { return parseEnum(s, ReplayResultMap) }
func ExpectReplayResult[S stringLike](s S) ReplayResult         { return expectEnum(s, ReplayResultMap) }

type ReplayCause int

const (
	_ ReplayCause = iota
	Checkmate
	Forfeit
	Stalemate
)

var replayCauseEntries = []enumEntry[ReplayCause]{
	{Checkmate, "CHECKMATE"},
	{Forfeit, "FORFEIT"},
	{Stalemate, "STALEMATE"},
}

var ReplayCauseMap = buildReverseMap(replayCauseEntries)

func (c ReplayCause) String() string                { return enumString(c, replayCauseEntries) }
func (c ReplayCause) MarshalJSON() ([]byte, error)  { return marshalEnum(c, replayCauseEntries) }
func (c *ReplayCause) UnmarshalJSON(d []byte) error { return unmarshalEnum(d, replayCauseEntries, c) }

func ParseReplayCause[S stringLike](s S) (ReplayCause, error) { return parseEnum(s, ReplayCauseMap) }
func ExpectReplayCause[S stringLike](s S) ReplayCause         { return expectEnum(s, ReplayCauseMap) }

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

var gameModeEntries = []enumEntry[GameMode]{
	{ModeTimed1Plus0, "TIMED_1+0"},
	{ModeTimed3Plus2, "TIMED_3+2"},
	{ModeTimed15Plus10, "TIMED_15+10"},
	{ModeCorrespondence1, "CORRESPONDENCE_1"},
	{ModeCorrespondence7, "CORRESPONDENCE_7"},
	{ModeCorrespondence14, "CORRESPONDENCE_14"},
}

var GameModeMap = buildReverseMap(gameModeEntries)

var RealTimeModes = []GameMode{ModeTimed1Plus0, ModeTimed3Plus2, ModeTimed15Plus10}

func (m GameMode) IsRealTime() bool { return slices.Contains(RealTimeModes, m) }

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

func (m GameMode) String() string                { return enumString(m, gameModeEntries) }
func (m GameMode) MarshalJSON() ([]byte, error)  { return marshalEnum(m, gameModeEntries) }
func (m *GameMode) UnmarshalJSON(d []byte) error { return unmarshalEnum(d, gameModeEntries, m) }

func ParseGameMode[S stringLike](s S) (GameMode, error) { return parseEnum(s, GameModeMap) }
func ExpectGameMode[S stringLike](s S) GameMode         { return expectEnum(s, GameModeMap) }

type Color int

const (
	Random Color = iota
	White
	Black
)

var colorEntries = []enumEntry[Color]{
	{Random, "RANDOM"},
	{White, "WHITE"},
	{Black, "BLACK"},
}

var ColorMap = buildReverseMap(colorEntries)

func (c Color) String() string                { return enumString(c, colorEntries) }
func (c Color) MarshalJSON() ([]byte, error)  { return marshalEnum(c, colorEntries) }
func (c *Color) UnmarshalJSON(d []byte) error { return unmarshalEnum(d, colorEntries, c) }

func ParseColor[S stringLike](s S) (Color, error) { return parseEnum(s, ColorMap) }
func ExpectColor[S stringLike](s S) Color         { return expectEnum(s, ColorMap) }

type TournamentStatus int

const (
	_ TournamentStatus = iota
	TournamentLobby
	TournamentInProgress
	TournamentFinished
)

var tournamentStatusEntries = []enumEntry[TournamentStatus]{
	{TournamentLobby, "LOBBY"},
	{TournamentInProgress, "IN_PROGRESS"},
	{TournamentFinished, "FINISHED"},
}

var TournamentStatusMap = buildReverseMap(tournamentStatusEntries)

func (t TournamentStatus) String() string { return enumString(t, tournamentStatusEntries) }
func (t TournamentStatus) MarshalJSON() ([]byte, error) {
	return marshalEnum(t, tournamentStatusEntries)
}
func (t *TournamentStatus) UnmarshalJSON(d []byte) error {
	return unmarshalEnum(d, tournamentStatusEntries, t)
}

func ParseTournamentStatus[S stringLike](s S) (TournamentStatus, error) {
	return parseEnum(s, TournamentStatusMap)
}
func ExpectTournamentStatus[S stringLike](s S) TournamentStatus {
	return expectEnum(s, TournamentStatusMap)
}
