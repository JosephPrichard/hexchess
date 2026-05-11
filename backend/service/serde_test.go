package svc

import (
	"encoding/json"
	"hexchess-svc/model"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestChessSerializer(t *testing.T) {
	t.Parallel()

	input1 := MakeChessState(StateSetup{ID: uuid.NewString(), Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	input1.EndState = Finished

	input2 := MakeChessState(StateSetup{ID: uuid.NewString(), Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	input2.Game.InitPieceMoves()
	input2.Game.ClearTables() // since we're asserting the output back to the input, we must clear payload that isn't serialized

	tests := []struct {
		name  string
		state *ChessState
	}{
		{name: "EchoSerializeStateWithFinish", state: input1},
		{name: "EchoSerializeStateWithMoves", state: input2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := proto.Marshal(SerializeChessState(tt.state))
			if err != nil {
				t.Fatalf("marshal chess state: %v", err)
			}
			output, err := UnmarshalChessState(b)
			if err != nil {
				t.Fatalf("deserialize state: %v", err)
			}
			// t.Logf("deserialized state: %v, board: %v", output, output.Game.Board.String())
			assert.Equal(t, tt.state, output)
		})
	}
}

func BenchmarkProtoChessSerializer(b *testing.B) {
	input := MakeChessState(StateSetup{ID: uuid.NewString(), Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	input.Game.InitPieceMoves()
	b.ResetTimer()
	for range b.N {
		v, err := proto.Marshal(SerializeChessState(input))
		if err != nil {
			b.Fatalf("marshal chess s: %v", err)
		}
		if _, err := UnmarshalChessState(v); err != nil {
			b.Fatalf("unmarshal s: %v", err)
		}
	}
}

func BenchmarkJsonChessSerializer(b *testing.B) {
	input := MakeChessState(StateSetup{ID: uuid.NewString(), Mode: model.ModeCorrespondence1, FirstColor: model.Random})
	input.Game.InitPieceMoves()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := json.Marshal(input)
		if err != nil {
			b.Fatalf("marshal s: %v", err)
		}
		var output ChessState
		if err := json.Unmarshal(v, &output); err != nil {
			b.Fatalf("unmarshal s: %v", err)
		}
	}
}
