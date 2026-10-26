package model

import (
	"errors"
	"hexchess-svc/chess"
	"hexchess-svc/utils/opt"
	"time"
)

type UndoState struct {
	UndoID int64 `json:"undoID"`
}

const EmptyUndoID = 0

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
	ID           GameID      `json:"id"`
	StartTime    time.Time   `json:"startTime"`
	WhitePlayer  PlayerState `json:"whitePlayer"`
	BlackPlayer  PlayerState `json:"blackPlayer"`
	FirstColor   GameColor   `json:"firstColor"`
	Mode         GameMode    `json:"mode"`
	EndState     EndKind     `json:"endState"`
	InitialBoard chess.Board `json:"-"`
	Game         chess.Game  `json:"-"`
}

type ChessMeta struct {
	GameID      GameID           `json:"gameId"`
	WhitePlayer opt.Option[User] `json:"whitePlayer"`
	BlackPlayer opt.Option[User] `json:"blackPlayer"`
	Mode        GameMode         `json:"mode"`
	Ordering    int64            `json:"ordering"`
}

type ChessStateArgs struct {
	ID           GameID
	Mode         GameMode
	FirstColor   GameColor
	White        PlayerState
	Black        PlayerState
	InitialBoard *chess.Board
	Game         *chess.Game
	EndState     EndKind
	UndoState    UndoState
}

func NewChessStateValue(args ChessStateArgs) ChessState {
	board := chess.NewStartBoard()
	if args.InitialBoard != nil {
		board = *args.InitialBoard
	}
	game := chess.Game{Board: board}
	if args.Game != nil {
		game = *args.Game
	}
	return ChessState{
		UndoState:    args.UndoState,
		ID:           args.ID,
		StartTime:    time.Now(),
		FirstColor:   args.FirstColor,
		Mode:         args.Mode,
		WhitePlayer:  args.White,
		BlackPlayer:  args.Black,
		EndState:     args.EndState,
		InitialBoard: board,
		Game:         game,
	}
}

func NewChessState(args ChessStateArgs) *ChessState {
	chessState := NewChessStateValue(args)
	return &chessState
}

var ErrNoMoveUndo = errors.New("no move to undo")

func (state *ChessState) String() string {
	if state != nil {
		return state.ID.String()
	}
	return "<nil>"
}

func (state *ChessState) HasBothPlayers() bool {
	return state.WhitePlayer.Present && state.BlackPlayer.Present
}

func (state *ChessState) IsEitherPlayer(player PlayerState) bool {
	return state.WhitePlayer.IsSame(player) || state.BlackPlayer.IsSame(player)
}

func (state *ChessState) ClockTimeout() time.Time {
	//clockTime := state.Mode.TotalTime()
	//
	//for i, move := range state.Game.Moves {
	//	if i%2 == 0 && state.Game.Board.IsWhiteTurn {
	//		incr := state.Mode.TimeIncrement()
	//
	//		clockTime += incr
	//	}
	//}
	//
	//return state.StartTime.Add(clockTime)
	return time.Time{}
}

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
