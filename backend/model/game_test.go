package model

import (
	"hexchess-svc/chess"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUndo(t *testing.T) {
	t.Run("no moves to undo", func(t *testing.T) {
		s := NewChessState(StateSetup{
			ID:           "test",
			Game:         new(chess.NewStartGame()),
			InitialBoard: new(chess.InitialBoard()),
		})

		err := s.Undo()

		assert.Equal(t, ErrNoMoveUndo, err)
	})

	t.Run("successfully undoing game with one move", func(t *testing.T) {
		game := chess.NewStartGame()
		game.Moves = append(game.Moves, game.NewMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")}))

		s := NewChessState(StateSetup{
			ID:           "test",
			Game:         new(game),
			InitialBoard: new(chess.InitialBoard()),
		})

		err := s.Undo()

		require.NoError(t, err)
		assert.Len(t, s.Game.Moves, 0)
	})
}
