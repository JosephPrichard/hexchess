package svc

import (
	"errors"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/chess"
	"math/rand/v2" // concurrency safe
	"time"
)

type PlayerState struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Elo     float64 `json:"elo"`
	IsGuest bool    `json:"isGuest"`
	Present bool    `json:"present"`
}

// Use the constructor functions to create games so the boolean flags will be properly initialized - as opposed to remembering to flag them

func MakeGuest() PlayerState {
	// concurrency safe to use rand - we are also using random negative integers for guests so we will never have a collision with an actual player
	return PlayerState{ID: -rand.Int64(), Name: "Guest", IsGuest: true, Present: true}
}

func MakeIDPlayer(id int64) PlayerState {
	return PlayerState{ID: id, Present: true}
}

func MakeNamePlayer(id int64, name string) PlayerState {
	return PlayerState{ID: id, Name: name, Present: true}
}

func MakePlayer(id int64, name string, country string) PlayerState {
	return PlayerState{ID: id, Name: name, Country: country, Present: true}
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
	ID          string      `json:"id"`
	WhitePlayer PlayerState `json:"whitePlayer"`
	BlackPlayer PlayerState `json:"blackPlayer"`
	IsEnded     bool        `json:"isEnded"`
	FirstColor  Color       `json:"firstColor"`
	Mode        GameMode    `json:"mode"`
	Touch       time.Time   `json:"touch"`
}

var ChessMetaCmpOpts = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	Mode         GameMode
	FirstColor   Color
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
	var whitePlayer, blackPlayer PlayerState
	if s.White != nil {
		whitePlayer = *s.White
	}
	if s.Black != nil {
		blackPlayer = *s.Black
	}
	state := ChessState{
		InitialBoard: b,
		Game:         game,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			Mode:        s.Mode,
			Touch:       time.UnixMilli(0),
			WhitePlayer: whitePlayer,
			BlackPlayer: blackPlayer,
		},
	}
	return state
}

func (s *ChessState) CurrPlayer() PlayerState {
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
			ID:         s.ID,
			IsEnded:    s.IsEnded,
			FirstColor: s.FirstColor,
			Mode:       s.Mode,
			Touch:      s.Touch,
		},
	}
	s2.WhitePlayer = s.WhitePlayer
	s2.BlackPlayer = s.BlackPlayer
	return s2
}
