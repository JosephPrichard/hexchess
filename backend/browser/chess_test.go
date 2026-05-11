//go:build browser

package browser

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/hexchess"
	"syscall/js"
	"testing"

	"hexchess-svc/pb"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func requireNoError(t *testing.T, err error) {
	// helper to replicate the logic of 'requireNoError' since it does not work in the browser
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func makeTestWasm() *ChessWasm {
	return &ChessWasm{Global: js.Global(), Version: "debug"}
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
	board := hexchess.InitialBoard()
	bytes, err := proto.Marshal(hexchess.SerializeBoard(&board))
	requireNoError(t, err)
	return uint8ArrayFromBytes(bytes)
}

func jsValueToGame(t *testing.T, result js.Value) hexchess.Game {
	gameOut := make([]byte, result.Length())
	js.CopyBytesToGo(gameOut, result)

	var pbGame pb.ChessGame
	requireNoError(t, proto.Unmarshal(gameOut, &pbGame))

	game, err := hexchess.DeserializeGame(&pbGame)
	requireNoError(t, err)
	return game
}

func assertGame(t *testing.T, wantGame *hexchess.Game, result js.Value) {
	t.Helper()
	if wantGame != nil {
		if !isUint8Array(result) {
			t.Fatalf("expected return value to be Uint8Array, got %v", result.Type())
		}

		game := jsValueToGame(t, result)
		diff := cmp.Diff(&game, wantGame, cmpopts.IgnoreFields(hexchess.HistMove{}, "WhiteTimer", "BlackTimer"))
		if diff != "" {
			t.Fatalf("expected games to be equal:\n%s", diff)
		}
	} else {
		if result.Type() != js.TypeUndefined {
			t.Fatalf("expected return value to be undefined, got %s", result.Type())
		}
	}
}

func TestGetGame(t *testing.T) {
	t.Parallel()

	wasm := makeTestWasm()

	t.Run("get initial game", func(t *testing.T) {
		t.Parallel()

		result := wasm.GetGame(js.Undefined(), []js.Value{js.Undefined()}).(js.Value)

		if !isUint8Array(result) {
			t.Fatal("expected return value to be Uint8Array")
		}
		game := jsValueToGame(t, result)

		wantBoard := hexchess.InitialBoard()
		diff := cmp.Diff(game.Board, wantBoard)
		if diff != "" {
			t.Fatalf("expected boards to be equal:\n%s", diff)
		}
	})

	t.Run("get game with moves", func(t *testing.T) {
		t.Parallel()

		input := makeInitialBoardJs(t)

		result := wasm.GetGame(js.Undefined(), []js.Value{input}).(js.Value)

		if !isUint8Array(result) {
			t.Fatal("expected return value to be Uint8Array")
		}
		game := jsValueToGame(t, result)

		wantGame := hexchess.Game{Board: hexchess.InitialBoard()}
		wantGame.InitPieceMoves() // GetGame will just call this.
		wantGame.ClearTables()    // not serialized.

		diff := cmp.Diff(game, wantGame)
		if diff != "" {
			t.Fatalf("expected games to be equal:\n%s", diff)
		}
	})
}

func TestMakeMove(t *testing.T) {
	t.Parallel()

	wasm := makeTestWasm()

	t.Run("successfully make move", func(t *testing.T) {
		t.Parallel()

		gameBytes, err := proto.Marshal(
			hexchess.SerializeGame(&hexchess.Game{Board: hexchess.InitialBoard()}),
		)
		requireNoError(t, err)

		moveBytes, err := proto.Marshal(&pb.Move{
			FromFile: 1, FromRank: 0,
			ToFile: 1, ToRank: 1,
		})
		requireNoError(t, err)

		inputs := []js.Value{uint8ArrayFromBytes(gameBytes), uint8ArrayFromBytes(moveBytes)}

		result := wasm.MakeMove(js.Undefined(), inputs).(js.Value)

		wantGame := &hexchess.Game{Board: hexchess.InitialBoard()}
		wantGame.InitPieceMoves() // used when calculating histories
		wantGame.MakeHistMove(hexchess.Move{
			From: hexchess.HexStr("b1"),
			To:   hexchess.HexStr("b2"),
		})
		wantGame.InitPieceMoves() // wasm MakeMove regenerates moves
		wantGame.ClearTables()    // not serialized

		assertGame(t, wantGame, result)
	})

	t.Run("invalid make move", func(t *testing.T) {
		t.Parallel()

		gameBytes, err := proto.Marshal(
			hexchess.SerializeGame(&hexchess.Game{Board: hexchess.InitialBoard()}),
		)
		requireNoError(t, err)

		moveBytes, err := proto.Marshal(&pb.Move{})
		requireNoError(t, err)

		inputs := []js.Value{uint8ArrayFromBytes(gameBytes), uint8ArrayFromBytes(moveBytes)}

		result := wasm.MakeMove(js.Undefined(), inputs).(js.Value)

		assertGame(t, nil, result)
	})

	t.Run("invalid arguments", func(t *testing.T) {
		t.Parallel()

		result := wasm.MakeMove(js.Undefined(), nil).(js.Value)
		assertGame(t, nil, result)
	})
}

func TestFenToGame(t *testing.T) {
	t.Parallel()

	wasm := makeTestWasm()

	for _, test := range []struct {
		name     string
		fen      string
		wantGame *hexchess.Game
		wantErr  string
	}{
		{
			name: "initial position",
			fen:  "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w",
			wantGame: func() *hexchess.Game {
				g := &hexchess.Game{Board: hexchess.InitialBoard()}
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
			result := wasm.FenToGame(js.Undefined(), []js.Value{js.ValueOf(test.fen)}).(js.Value)

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
					t.Fatalf("expected error=%s, got=%s", test.wantErr, ret1.String())
				}
			}
		})
	}
}

func TestBoardToFen(t *testing.T) {
	t.Parallel()

	wasm := makeTestWasm()

	input := makeInitialBoardJs(t)

	result := wasm.BoardToFen(js.Undefined(), []js.Value{input}).(js.Value)

	if result.Type() != js.TypeString {
		t.Fatalf("expected string, got %s", result.Type())
	}
	wantFen := "6/P5p/RP4pr/N1P3p1n/Q2P2p2q/BBB1P1p1bbb/K2P2p2k/N1P3p1n/RP4pr/P5p/6 w"
	if wantFen != result.String() {
		t.Fatalf("expected string to be %s, got %s", wantFen, result.String())
	}
}

func TestGameAtMoveIndex(t *testing.T) {
	t.Parallel()

	wasm := makeTestWasm()

	t.Run("jump to index 0", func(t *testing.T) {
		t.Parallel()

		game := &hexchess.Game{Board: hexchess.InitialBoard()}
		game.MakeHistMove(hexchess.Move{From: hexchess.HexStr("b1"), To: hexchess.HexStr("b2")})
		game.MakeHistMove(hexchess.Move{From: hexchess.HexStr("b7"), To: hexchess.HexStr("b6")})

		movesBytes, err := proto.Marshal(&pb.HistMoves{Moves: hexchess.SerializeMoveList(game.Moves)})
		requireNoError(t, err)

		inputs := []js.Value{makeInitialBoardJs(t), uint8ArrayFromBytes(movesBytes), js.ValueOf(0)}

		result := wasm.GameAtMoveIndex(js.Undefined(), inputs).(js.Value)

		wantGame := &hexchess.Game{Board: hexchess.InitialBoard()}
		wantGame.MakeHistMove(hexchess.Move{From: hexchess.HexStr("b1"), To: hexchess.HexStr("b2")})
		wantGame.InitPieceMoves()
		wantGame.ClearTables()

		assertGame(t, wantGame, result)
	})

	t.Run("invalid move index", func(t *testing.T) {
		t.Parallel()

		movesBytes, err := proto.Marshal(&pb.HistMoves{})
		requireNoError(t, err)

		inputs := []js.Value{makeInitialBoardJs(t), uint8ArrayFromBytes(movesBytes), js.ValueOf(10)}

		result := wasm.GameAtMoveIndex(js.Undefined(), inputs).(js.Value)

		assertGame(t, nil, result)
	})

	t.Run("invalid arguments", func(t *testing.T) {
		t.Parallel()

		result := wasm.GameAtMoveIndex(js.Undefined(), nil).(js.Value)
		assertGame(t, nil, result)
	})
}
