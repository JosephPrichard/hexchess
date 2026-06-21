package model

import (
	"hexchess-svc/chess"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUndo(t *testing.T) {
	t.Parallel()

	t.Run("no moves to undo", func(t *testing.T) {
		t.Parallel()

		s := NewChessState(StateSetup{
			ID:           "test",
			Game:         ptr(chess.NewStartGame()),
			InitialBoard: ptr(chess.InitialBoard()),
		})

		err := s.Undo()

		assert.Equal(t, ErrNoMoveUndo, err)
	})

	t.Run("successfully undoing game with one move", func(t *testing.T) {
		t.Parallel()

		game := chess.NewStartGame()
		game.Moves = append(game.Moves, game.NewMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")}))

		s := NewChessState(StateSetup{
			ID:           "test",
			Game:         ptr(game),
			InitialBoard: ptr(chess.InitialBoard()),
		})

		err := s.Undo()

		require.NoError(t, err)
		assert.Len(t, s.Game.Moves, 0)
	})
}
