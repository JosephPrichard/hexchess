package data

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChessSerializer(t *testing.T) {
	input1 := MakeState(uuid.NewString(), RealTime)
	input2 := MakeState(uuid.NewString(), RealTime)
	input2.Game.InitPieceMoves()

	inputs := []ChessState{input1, input2}

	for i, input := range inputs {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			b, err := MarshalChessState(input)
			if err != nil {
				t.Fatalf("failed to serialize state: %v", err)
			}

			output, err := UnmarshalChess(b)
			if err != nil {
				t.Fatalf("failed to deserialize state: %v", err)
			}

			t.Logf("deserialized state: %v, board: %v", output, output.Game.Board.String())

			assert.Equal(t, input, output)
		})
	}
}
