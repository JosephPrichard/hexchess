package data

import (
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/chess"
	"time"
)

type PlayerState struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Elo     float64 `json:"elo"`
	IsGuest bool    `json:"isGuest"`
}

type ChessState struct {
	ChessMeta
	InitialBoard chess.Board
	Game         chess.Game
}

type ChessMeta struct {
	ID          string       `json:"id"`
	WhitePlayer *PlayerState `json:"whitePlayer"`
	BlackPlayer *PlayerState `json:"blackPlayer"`
	IsEnded     bool         `json:"isEnded"`
	FirstColor  ColorSelect  `json:"firstColor"`
	TimeControl TimeControl  `json:"timeControl"`
	Touch       time.Time    `json:"touch"`
}

var ChessMetaCmpOpts = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

func MakeState(id string, timeControl TimeControl) ChessState {
	return ChessState{
		InitialBoard: chess.MakeStartBoard(),
		Game:         chess.MakeStartGame(),
		ChessMeta: ChessMeta{
			ID:          id,
			FirstColor:  Random,
			TimeControl: timeControl,
			Touch:       time.UnixMilli(0),
		},
	}
}

func MakeStateWithPlayers(id string, timeControl TimeControl, whitePlayer *PlayerState, blackPlayer *PlayerState) ChessState {
	return ChessState{
		Game: chess.MakeStartGame(),
		ChessMeta: ChessMeta{
			ID:          id,
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
			FirstColor:  Random,
			TimeControl: timeControl,
			Touch:       time.UnixMilli(0),
		},
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
		Game: s.Game.DeepCopy(),
		ChessMeta: ChessMeta{
			ID:          s.ID,
			IsEnded:     s.IsEnded,
			FirstColor:  s.FirstColor,
			TimeControl: s.TimeControl,
			Touch:       s.Touch,
		},
	}

	if s.WhitePlayer != nil {
		s2.WhitePlayer = &(*s.WhitePlayer)
	}
	if s.BlackPlayer != nil {
		s2.BlackPlayer = &(*s.BlackPlayer)
	}

	return s2
}
