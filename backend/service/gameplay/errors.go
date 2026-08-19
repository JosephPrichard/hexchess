package gameplay

import (
	"errors"
	"fmt"
	"hexchess-svc/model"
)

var ErrForfeitPlayer = errors.New("must be a player to forfeit or abort")

type ErrStartedGame struct {
	GameID model.GameID
}

func (e ErrStartedGame) Error() string {
	return fmt.Sprintf("does not have both players (gameId=%s)", e.GameID)
}

type ErrFinishedGame struct {
	GameID model.GameID
}

func (e ErrFinishedGame) Error() string {
	return fmt.Sprintf("attempted on ended game gameId=%s", e.GameID)
}

type ErrTurn struct {
	GameID   model.GameID
	PlayerID int64
	CurrID   int64
}

func (e ErrTurn) Error() string {
	return fmt.Sprintf("invalid turn (player=%d, curr=%d, game=%s)", e.PlayerID, e.CurrID, e.GameID)
}

type ErrTimeout struct {
	GameID model.GameID
}

func (e ErrTimeout) Error() string {
	return fmt.Sprintf("game timer has expired (game=%s)", e.GameID)
}

type ErrInvalidMove struct {
	GameID    model.GameID
	PlayerID  int64
	Violation error
}

func (e ErrInvalidMove) Error() string {
	return fmt.Sprintf("invalid move (violation=%v, player=%d, game=%s)", e.Violation, e.PlayerID, e.GameID)
}
