package data

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestChessSerializer(t *testing.T) {
	input1 := MakeState(uuid.NewString(), RealTime)
	input2 := MakeState(uuid.NewString(), RealTime)
	input2.Game.InitPieceMoves()
	input2.Game.ClearTables() // since we're asserting the output back to the input, we must clear data that isn't serialized

	inputs := []ChessState{input1, input2}

	for i, input := range inputs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
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

func BenchmarkProtoChessSerializer(b *testing.B) {
	input := MakeState(uuid.NewString(), RealTime)
	input.Game.InitPieceMoves()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := MarshalChessState(input)
		if err != nil {
			b.Fatalf("failed to marshal state: %v", err)
		}
		if _, err := UnmarshalChess(v); err != nil {
			b.Fatalf("failed to unmarshal state: %v", err)
		}
	}
}

func BenchmarkJsonChessSerializer(b *testing.B) {
	input := MakeState(uuid.NewString(), RealTime)
	input.Game.InitPieceMoves()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := json.Marshal(input)
		if err != nil {
			b.Fatalf("failed to marshal state: %v", err)
		}
		var output ChessState
		if err := json.Unmarshal(v, &output); err != nil {
			b.Fatalf("failed to unmarshal state: %v", err)
		}
	}
}
