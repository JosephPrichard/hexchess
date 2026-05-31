package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/chess"
	"testing"
)

func TestUndo(t *testing.T) {
	t.Parallel()

	t.Run("no moves to undo", func(t *testing.T) {
		t.Parallel()

		s := MakeChessState(StateSetup{
			ID:           "test",
			Game:         ptr(chess.MakeStartGame()),
			InitialBoard: ptr(chess.InitialBoard()),
		})

		err := s.Undo()

		assert.Equal(t, ErrNoMoveUndo, err)
	})

	t.Run("successfully undoing game with one move", func(t *testing.T) {
		t.Parallel()

		game := chess.MakeStartGame()
		game.Moves = append(game.Moves, game.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")}))

		s := MakeChessState(StateSetup{
			ID:           "test",
			Game:         ptr(game),
			InitialBoard: ptr(chess.InitialBoard()),
		})

		err := s.Undo()

		require.NoError(t, err)
		assert.Len(t, s.Game.Moves, 0)
	})
}
