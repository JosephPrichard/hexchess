package game

import (
	"encoding/json/v2"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
)

var chessStateCmpOpts = cmpopts.IgnoreFields(ChessState{}, "StartTime")

func TestChessSerializer(t *testing.T) {
	input1 := NewChessState(ChessStateArgs{ID: gameid.NewGameID(), Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
	input1.EndState = Finished

	input2 := NewChessState(ChessStateArgs{ID: gameid.NewGameID(), Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
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
			bytes, err := MarshalChessState(tt.state)
			if err != nil {
				t.Fatalf("marshal chess state: %v", err)
			}
			output, err := UnmarshalChessState(bytes)
			if err != nil {
				t.Fatalf("deserialize state: %v", err)
			}
			// t.Logf("deserialized state: %v, board: %v", output, output.Game.Board.String())
			testutil.Equal(t, tt.state, output, chessStateCmpOpts)
		})
	}
}

func BenchmarkProtoChessSerializer(b *testing.B) {
	input := NewChessState(ChessStateArgs{ID: gameid.NewGameID(), Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
	input.Game.InitPieceMoves()
	b.ResetTimer()
	for range b.N {
		bytes, err := MarshalChessState(input)
		if err != nil {
			b.Fatalf("marshal chess state: %v", err)
		}
		if _, err := UnmarshalChessState(bytes); err != nil {
			b.Fatalf("unmarshal state: %v", err)
		}
	}

	bytes, err := MarshalChessState(input)
	if err != nil {
		b.Fatalf("marshal chess state: %v", err)
	}
	b.Logf("length of marshalled chess state: %v bytes", len(bytes))
}

func BenchmarkJsonChessSerializer(b *testing.B) {
	input := NewChessState(ChessStateArgs{ID: gameid.NewGameID(), Mode: gamedata.ModeCorrespondence1, FirstColor: gamedata.Random})
	input.Game.InitPieceMoves()
	b.ResetTimer()
	for range b.N {
		bytes, err := json.Marshal(input)
		if err != nil {
			b.Fatalf("marshal state: %v", err)
		}
		var output ChessState
		if err := json.Unmarshal(bytes, &output); err != nil {
			b.Fatalf("unmarshal state: %v", err)
		}
	}

	bytes, err := json.Marshal(input)
	if err != nil {
		b.Fatalf("marshal chess state: %v", err)
	}
	b.Logf("length of marshalled chess state: %v bytes", len(bytes))
}
