package svc

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"strconv"
	"testing"
)

func TestChessSerializer(t *testing.T) {
	input1 := MakeState(StateSetup{ID: uuid.NewString(), TimeControl: TcRealTime, FirstColor: ColorRandom})
	input2 := MakeState(StateSetup{ID: uuid.NewString(), TimeControl: TcRealTime, FirstColor: ColorRandom})
	input2.Game.InitPieceMoves()
	input2.Game.ClearTables() // since we're asserting the output back to the input, we must clear data that isn't serialized

	for i, input := range []ChessState{
		input1,
		input2,
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			b, err := proto.Marshal(SerializeChessState(input))
			if err != nil {
				t.Fatalf("marshal chess state: %v", err)
			}
			output, err := UnmarshalChessState(b)
			if err != nil {
				t.Fatalf("deserialize state: %v", err)
			}

			t.Logf("deserialized state: %v, board: %v", output, output.Game.Board.String())
			assert.Equal(t, input, output)
		})
	}
}

func BenchmarkProtoChessSerializer(b *testing.B) {
	input := MakeState(StateSetup{ID: uuid.NewString(), TimeControl: TcRealTime, FirstColor: ColorRandom})
	input.Game.InitPieceMoves()

	b.ResetTimer()
	for range b.N {
		v, err := proto.Marshal(SerializeChessState(input))
		if err != nil {
			b.Fatalf("marshal chess state: %v", err)
		}
		if _, err := UnmarshalChessState(v); err != nil {
			b.Fatalf("unmarshal state: %v", err)
		}
	}
}

func BenchmarkJsonChessSerializer(b *testing.B) {
	input := MakeState(StateSetup{ID: uuid.NewString(), TimeControl: TcRealTime, FirstColor: ColorRandom})
	input.Game.InitPieceMoves()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := json.Marshal(input)
		if err != nil {
			b.Fatalf("marshal state: %v", err)
		}
		var output ChessState
		if err := json.Unmarshal(v, &output); err != nil {
			b.Fatalf("unmarshal state: %v", err)
		}
	}
}
