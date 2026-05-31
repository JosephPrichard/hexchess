package model

import (
	"errors"
	"hexchess-svc/chess"
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

func (kind EndKind) IsEnded() bool {
	return kind != NotEnded
}

type ChessState struct {
	UndoState
	ID           string      `json:"id"`
	WhitePlayer  PlayerState `json:"whitePlayer"`
	BlackPlayer  PlayerState `json:"blackPlayer"`
	FirstColor   GameColor   `json:"firstColor"`
	Mode         GameMode    `json:"mode"`
	EndState     EndKind
	InitialBoard chess.Board
	Game         chess.Game
}

func (state *ChessState) HasBothPlayers() bool {
	return state.WhitePlayer.Present && state.BlackPlayer.Present
}

func (state *ChessState) IsEitherPlayer(player PlayerState) bool {
	return state.WhitePlayer.IsSame(player) || state.BlackPlayer.IsSame(player)
}

type ChessMeta struct {
	GameID      string   `json:"gameid"`
	WhitePlayer User     `json:"whitePlayer"`
	BlackPlayer User     `json:"blackPlayer"`
	Mode        GameMode `json:"mode"`
	Ordering    int64    `json:"ordering"`
}

type StateSetup struct {
	ID           string
	Mode         GameMode
	FirstColor   GameColor
	White        PlayerState
	Black        PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
	EndState     EndKind
	UndoState    UndoState
}

func MakeChessStateVal(s StateSetup) ChessState {
	board := chess.MakeStartBoard()
	if s.InitialBoard != nil {
		board = *s.InitialBoard
	}
	game := chess.Game{Board: board}
	if s.Game != nil {
		game = *s.Game
	}
	return ChessState{
		InitialBoard: board,
		Game:         game,
		UndoState:    s.UndoState,
		ID:           s.ID,
		FirstColor:   s.FirstColor,
		Mode:         s.Mode,
		WhitePlayer:  s.White,
		BlackPlayer:  s.Black,
		EndState:     s.EndState,
	}
}

func MakeChessState(s StateSetup) *ChessState {
	chessState := &ChessState{}
	*chessState = MakeChessStateVal(s)
	return chessState
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

func (state *ChessState) CurrPlayer() PlayerState {
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
		ID:           state.ID,
		FirstColor:   state.FirstColor,
		Mode:         state.Mode,
		EndState:     state.EndState,
	}
	s2.WhitePlayer = state.WhitePlayer
	s2.BlackPlayer = state.BlackPlayer
	return s2
}
