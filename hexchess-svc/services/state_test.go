package svc

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/chess"
	"testing"
)

func TestChessState_UndoMove(t *testing.T) {
	for _, test := range []struct {
		name      string
		setup     func() *ChessState
		wantErr   error
		wantMoves int
	}{
		{
			name: "no moves to undo",
			setup: func() *ChessState {
				return &ChessState{InitialBoard: chess.MakeStartBoard(), Game: chess.MakeStartGame()}
			},
			wantErr:   ErrNoMoveUndo,
			wantMoves: 0,
		},
		{
			name: "successfully undo-ing move",
			setup: func() *ChessState {
				state := &ChessState{
					InitialBoard: chess.MakeStartBoard(),
					Game:         chess.MakeStartGame(),
				}
				state.Game.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")})
				return state
			},
			wantErr:   nil,
			wantMoves: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cs := test.setup()

			err := cs.UndoMove()

			if test.wantErr != nil {
				assert.Equal(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, cs.Game.Moves, test.wantMoves)
		})
	}
}
