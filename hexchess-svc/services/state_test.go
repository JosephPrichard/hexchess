package svc

import (
	"github.com/stretchr/testify/assert"
	"hexchess-svc/chess"
	"strconv"
	"testing"
)

func TestChessState_UndoMove(t *testing.T) {
	tests := []struct {
		setup    func() *ChessState
		expErr   error
		expMoves int
	}{
		{
			setup: func() *ChessState {
				return &ChessState{
					InitialBoard: chess.MakeStartBoard(),
					Game:         chess.MakeStartGame(),
				}
			},
			expErr:   ErrNoMoveUndo,
			expMoves: 0,
		},
		{
			setup: func() *ChessState {
				cs := &ChessState{
					InitialBoard: chess.MakeStartBoard(),
					Game:         chess.MakeStartGame(),
				}
				cs.Game.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")})
				return cs
			},
			expErr:   nil,
			expMoves: 0,
		},
	}

	for tt, test := range tests {
		t.Run(strconv.Itoa(tt), func(t *testing.T) {
			cs := test.setup()

			err := cs.UndoMove()

			if test.expErr != nil {
				assert.Equal(t, err, test.expErr)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, cs.Game.Moves, test.expMoves)
		})
	}
}
