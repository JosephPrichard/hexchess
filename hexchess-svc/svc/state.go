package svc

import (
	"errors"
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

type UndoState struct {
	UndoID int64
}

type ChessState struct {
	ChessMeta
	UndoState
	InitialBoard chess.Board
	Game         chess.Game
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (s *ChessState) UndoMove() error {
	if len(s.Game.Moves) == 0 {
		return ErrNoMoveUndo
	}
	undoGame := chess.Game{Board: s.InitialBoard}
	movesExceptLast := s.Game.Moves[:len(s.Game.Moves)-1]
	for _, move := range movesExceptLast {
		undoGame.MakeMove(chess.Move{From: move.To, To: move.From, Promotion: move.Promotion})
	}
	s.Game = undoGame
	s.Game.InitPieceMoves()
	return nil
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

func (m *ChessMeta) GetWhiteID() int64 {
	if m == nil || m.WhitePlayer == nil {
		return -1
	}
	return m.WhitePlayer.ID
}

func (m *ChessMeta) GetBlackID() int64 {
	if m == nil || m.BlackPlayer == nil {
		return -1
	}
	return m.BlackPlayer.ID
}

var ChessMetaCmpOpts = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	TimeControl  TimeControl
	FirstColor   ColorSelect
	White        *PlayerState
	Black        *PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
}

func MakeState(s StateSetup) ChessState {
	b := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		b = *s.InitialBoard
	}
	game := chess.Game{Board: b}
	if s.Game != nil {
		game = *s.Game
	}
	state := ChessState{
		InitialBoard: b,
		Game:         game,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			TimeControl: s.TimeControl,
			Touch:       time.UnixMilli(0),
			WhitePlayer: s.White,
			BlackPlayer: s.Black,
		},
	}
	return state
}

func (s *ChessState) CurrPlayer() *PlayerState {
	if s.Game.Board.IsWhiteTurn {
		return s.WhitePlayer
	}
	return s.BlackPlayer
}

func (s *ChessState) DeepCopy() ChessState {
	s2 := ChessState{
		Game:         s.Game.DeepCopy(),
		UndoState:    s.UndoState,
		InitialBoard: s.InitialBoard,
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
