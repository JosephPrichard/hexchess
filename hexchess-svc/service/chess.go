package svc

import (
	"errors"
	"github.com/google/go-cmp/cmp/cmpopts"
	"hexchess-svc/chess"
	"hexchess-svc/domain"
	"time"
)

type UndoState struct {
	UndoID int64
}

type EndKind int

const (
	NotEnded EndKind = iota
	Aborted
	Finished
)

func (kind EndKind) isEnded() bool {
	return kind != NotEnded
}

type ChessState struct {
	ChessMeta
	UndoState
	EndState     EndKind
	InitialBoard chess.Board
	Game         chess.Game
}

func (state *ChessState) HasBothPlayers() bool {
	return state.WhitePlayer.Present && state.BlackPlayer.Present
}

func (state *ChessState) IsEitherPlayer(player domain.PlayerState) bool {
	return state.WhitePlayer.IsSame(player) || state.BlackPlayer.IsSame(player)
}

type ChessMeta struct {
	ID          string             `json:"id"`
	WhitePlayer domain.PlayerState `json:"whitePlayer"`
	BlackPlayer domain.PlayerState `json:"blackPlayer"`
	FirstColor  domain.GameColor   `json:"firstColor"`
	Mode        domain.GameMode    `json:"mode"`
	Touch       time.Time          `json:"touch"`
}

var ChessMetaCmpOpt = cmpopts.IgnoreFields(ChessMeta{}, "Touch")

type StateSetup struct {
	ID           string
	Mode         domain.GameMode
	FirstColor   domain.GameColor
	White        domain.PlayerState
	Black        domain.PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
	EndState     EndKind
	UndoState    UndoState
}

func MakeChessState(s StateSetup) *ChessState {
	board := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		board = *s.InitialBoard
	}
	game := chess.Game{Board: board}
	if s.Game != nil {
		game = *s.Game
	}
	return &ChessState{
		InitialBoard: board,
		Game:         game,
		UndoState:    s.UndoState,
		ChessMeta: ChessMeta{
			ID:          s.ID,
			FirstColor:  s.FirstColor,
			Mode:        s.Mode,
			Touch:       time.UnixMilli(0),
			WhitePlayer: s.White,
			BlackPlayer: s.Black,
		},
		EndState: s.EndState,
	}
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (state *ChessState) Undo() error {
	if len(state.Game.Moves) == 0 {
		return ErrNoMoveUndo
	}
	index := len(state.Game.Moves) - 2 // last element minus one.
	game, err := chess.JumpMoveIndex(state.InitialBoard, state.Game.Moves, index)
	if game != nil {
		state.Game = game.DeepCopy()
	}
	return err
}

func (state *ChessState) CurrPlayer() domain.PlayerState {
	if state.Game.Board.IsWhiteTurn {
		return state.WhitePlayer
	}
	return state.BlackPlayer
}

func (state *ChessState) DeepCopy() ChessState {
	s2 := ChessState{
		Game:         state.Game.DeepCopy(),
		UndoState:    state.UndoState,
		InitialBoard: state.InitialBoard,
		ChessMeta: ChessMeta{
			ID:         state.ID,
			FirstColor: state.FirstColor,
			Mode:       state.Mode,
			Touch:      state.Touch,
		},
		EndState: state.EndState,
	}
	s2.WhitePlayer = state.WhitePlayer
	s2.BlackPlayer = state.BlackPlayer
	return s2
}
