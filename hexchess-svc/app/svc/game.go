package svc

import (
	"fmt"
	"hexchess-svc/app/chess"
	"strings"
	"time"
)

type ColorSelect int

const (
	White ColorSelect = iota
	Black
	Random
)

type TimeControl int

const (
	RealTime TimeControl = iota
	Correspondence
	Unlimited
)

type PlayerState struct {
	ID      int64
	Name    string
	Country string
	Elo     float32
	IsGuest bool
}

type ChessState struct {
	ID          string
	Game        chess.Game
	MoveList    []chess.PieceMove
	WhitePlayer *PlayerState
	BlackPlayer *PlayerState
	IsEnded     bool
	FirstColor  ColorSelect
	TimeControl TimeControl
	Touch       time.Time
}

type ChessView struct {
	ID          string
	WhitePlayer *PlayerState
	BlackPlayer *PlayerState
	IsEnded     bool
	FirstColor  ColorSelect
	TimeControl TimeControl
}

func MakeStartChessState(id string, timeControl TimeControl) ChessState {
	return ChessState{
		ID:          id,
		Game:        chess.MakeStartGame(),
		FirstColor:  Random,
		TimeControl: timeControl,
		Touch:       time.UnixMilli(0),
	}
}

func ParseColorSelect(value string) (ColorSelect, error) {
	switch strings.ToUpper(value) {
	case "WHITE":
		return White, nil
	case "BLACK":
		return Black, nil
	case "RANDOM":
		return Random, nil
	default:
		return 0, fmt.Errorf("unknown color select: %s", value)
	}
}

func ParseTimeControl(value string) (TimeControl, error) {
	switch strings.ToUpper(value) {
	case "REAL_TIME":
		return RealTime, nil
	case "CORRESPONDENCE":
		return Correspondence, nil
	case "UNLIMITED":
		return Unlimited, nil
	default:
		return 0, fmt.Errorf("unknown time control: %s", value)
	}
}
