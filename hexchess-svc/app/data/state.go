package data

import (
	"hexchess-svc/app/chess"
	"time"
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

func MakeStartChessState(id string, timeControl TimeControl) ChessState {
	return ChessState{
		ID:          id,
		Game:        chess.MakeStartGame(),
		FirstColor:  Random,
		TimeControl: timeControl,
		Touch:       time.UnixMilli(0),
	}
}

func (s *ChessState) CurrPlayer() *PlayerState {
	if s.Game.Board.IsWhiteTurn {
		return s.WhitePlayer
	}
	return s.BlackPlayer
}

func (s *ChessState) DeepCopy() ChessState {
	s2 := ChessState{
		ID:          s.ID,
		Game:        s.Game.DeepCopy(),
		IsEnded:     s.IsEnded,
		FirstColor:  s.FirstColor,
		TimeControl: s.TimeControl,
		Touch:       s.Touch,
	}

	if s.WhitePlayer != nil {
		s2.WhitePlayer = &(*s.WhitePlayer)
	}
	if s.BlackPlayer != nil {
		s2.BlackPlayer = &(*s.BlackPlayer)
	}
	s2.MoveList = append(s2.MoveList, s.MoveList...)

	return s2
}
