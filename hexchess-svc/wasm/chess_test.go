package wasm

import (
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"reflect"
	"syscall/js"
	"testing"
)

func requireNoError(t *testing.T, err error) {
	// helper to replicate the logic of 'requireNoError' since it does not work in the browser
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func makeTestWasm() *ChessWasm {
	return &ChessWasm{Global: js.Global()}
}

func uint8ArrayFromBytes(b []byte) js.Value {
	arr := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	return arr
}

func isUint8Array(v js.Value) bool {
	return v.InstanceOf(js.Global().Get("Uint8Array"))
}

func makeInitialBoardJs(t *testing.T) js.Value {
	board := chess.InitialBoard()
	bytes, err := proto.Marshal(chess.SerializeBoard(&board))
	requireNoError(t, err)
	return uint8ArrayFromBytes(bytes)
}

func jsValueToGame(t *testing.T, result js.Value) chess.Game {
	gameOut := make([]byte, result.Length())
	js.CopyBytesToGo(gameOut, result)

	var pbGame pb.ChessGame
	requireNoError(t, proto.Unmarshal(gameOut, &pbGame))

	game, err := chess.DeserializeGame(&pbGame)
	requireNoError(t, err)
	return game
}

func TestGetInitialGame(t *testing.T) {
	w := makeTestWasm()

	result := w.GetInitialGame(js.Undefined(), nil).(js.Value)

	if !isUint8Array(result) {
		t.Fatal("expected return value to be Uint8Array")
	}
	game := jsValueToGame(t, result)

	wantBoard := chess.InitialBoard()
	diff := cmp.Diff(game.Board, wantBoard)
	if diff != "" {
		t.Fatalf("expected boards to be equal:\n%s", diff)
	}
}

func TestMakeMove(t *testing.T) {
	w := makeTestWasm()

	for _, test := range []struct {
		name     string
		pbMoveIn *pb.MakeMoveInput
		wantGame *chess.Game
	}{
		{
			name: "successfully make move",
			pbMoveIn: &pb.MakeMoveInput{
				Game: chess.SerializeGame(&chess.Game{Board: chess.InitialBoard()}),
				Move: &pb.Move{FromFile: 1, FromRank: 0, ToFile: 1, ToRank: 1},
			},
			wantGame: func() *chess.Game {
				g := &chess.Game{Board: chess.InitialBoard()}
				g.InitPieceMoves()                                                       // Moves are used while calculating histories in chess.MakeMove
				g.MakeMove(chess.Move{From: chess.HexStr("b1"), To: chess.HexStr("b2")}) // Applying the same move on our assertion
				g.InitPieceMoves()                                                       // wasm MakeMove will generate moves after making the move
				g.ClearTables()                                                          // not serialized.
				return g
			}(),
		},
		{
			name: "invalid make move",
			pbMoveIn: &pb.MakeMoveInput{
				Game: chess.SerializeGame(&chess.Game{Board: chess.InitialBoard()}),
				Move: &pb.Move{},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			bytes, err := proto.Marshal(test.pbMoveIn)
			requireNoError(t, err)
			input := uint8ArrayFromBytes(bytes)

			result := w.MakeMove(js.Undefined(), []js.Value{input}).(js.Value)

			if test.wantGame != nil {
				if !isUint8Array(result) {
					t.Fatal("expected return value to be Uint8Array")
				}

				game := jsValueToGame(t, result)
				diff := cmp.Diff(&game, test.wantGame)
				if diff != "" {
					t.Fatalf("expected games to be equal:\n%s", diff)
				}
			} else {
				if result.Type() != js.TypeUndefined {
					t.Fatalf("expected return value to be undefined, got %s", result.Type())
				}
			}
		})
	}
}

func TestGetMoves(t *testing.T) {
	w := makeTestWasm()

	input := makeInitialBoardJs(t)

	result := w.GetMoves(js.Undefined(), []js.Value{input}).(js.Value)

	if !isUint8Array(result) {
		t.Fatal("expected return value to be Uint8Array")
	}
	game := jsValueToGame(t, result)

	wantGame := chess.Game{Board: chess.InitialBoard()}
	wantGame.InitPieceMoves() // GetMoves will just call this.
	wantGame.ClearTables()    // not serialized.

	diff := cmp.Diff(game, wantGame)
	if diff != "" {
		t.Fatalf("expected games to be equal:\n%s", diff)
	}
}

func TestFenToGame(t *testing.T) {
	w := makeTestWasm()

	for _, test := range []struct {
		name     string
		fen      string
		wantGame *chess.Game
		wantErr  string
	}{
		{
			name: "initial position",
			fen:  "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantGame: func() *chess.Game {
				g := &chess.Game{Board: chess.InitialBoard()}
				g.InitPieceMoves()
				g.ClearTables()
				return g
			}(),
		},
		{
			name:    "invalid FEN",
			fen:     "rubbish",
			wantErr: "invalid FEN character: u",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := w.FenToGame(js.Undefined(), []js.Value{js.ValueOf(test.fen)}).(js.Value)

			if result.Type() != js.TypeObject {
				t.Fatalf("expected return value to be array, got %s", result.Type())
			}

			if test.wantGame != nil {
				ret0 := result.Index(0)
				if !isUint8Array(ret0) {
					t.Fatalf("expected index 0 to be Uint8Array, got %s", result.Type())
				}
				game := jsValueToGame(t, ret0)

				diff := cmp.Diff(game, *test.wantGame)
				if diff != "" {
					t.Fatalf("expected board to be equal:\n%s", diff)
				}
			} else {
				ret1 := result.Index(1)
				if ret1.Type() != js.TypeString {
					t.Fatalf("expected index 0 to be string, got %s", result.Type())
				}

				if test.wantErr != ret1.String() {
					t.Fatalf("expected error '%s', got '%s'", test.wantErr, ret1.String())
				}
			}
		})
	}
}

func TestBoardToFen(t *testing.T) {
	w := makeTestWasm()

	input := makeInitialBoardJs(t)

	result := w.BoardToFen(js.Undefined(), []js.Value{input}).(js.Value)

	if result.Type() != js.TypeString {
		t.Fatalf("expected string, got %s", result.Type())
	}
	wantFen := "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w"
	if wantFen != result.String() {
		t.Fatalf("expected string to be %s, got %s", wantFen, result.String())
	}
}

func TestGetMoveNotations(t *testing.T) {
	w := makeTestWasm()

	histMoves := []chess.HistMove{{
		PieceMove: chess.PieceMove{Piece: chess.WhitePawn, From: chess.HexStr("k1"), To: chess.HexStr("f6")},
	}}
	bytes, err := proto.Marshal(&pb.HistMoves{Moves: chess.SerializeMoveList(histMoves)})
	requireNoError(t, err)
	input := uint8ArrayFromBytes(bytes)

	result := w.GetMoveNotations(js.Undefined(), []js.Value{input}).(js.Value)

	if result.Type() != js.TypeObject {
		t.Fatalf("expected array, got %s", result.Type())
	}

	var strs []string
	for i := range result.Length() {
		e := result.Index(i)
		if e.Type() != js.TypeString {
			t.Fatalf("expected element of return value to be string, got %s", e.Type())
		}
		strs = append(strs, e.String())
	}

	wantStrs := []string{"Pf6"}
	if !reflect.DeepEqual(wantStrs, strs) {
		t.Fatalf("expected %v, got %v", wantStrs, strs)
	}
}
