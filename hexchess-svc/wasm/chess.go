//go:build wasm

package wasm

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"strings"
	"syscall/js"
)

type ChessWasm struct {
	Global  js.Value
	Version string
}

func (w *ChessWasm) IsDebug() bool {
	return w.Version == "debug"
}

func (w *ChessWasm) JsLog(args ...any) {
	w.Global.Get("console").Call("log", args...)
}

func (w *ChessWasm) JsDebugLog(args ...string) {
	if w.IsDebug() {
		var logStr strings.Builder
		for _, arg := range args {
			logStr.WriteString(arg + "\n\n")
		}
		w.Global.Get("console").Call("warn", logStr.String())
	}
}

func (w *ChessWasm) JsErr(err error) js.Value {
	return w.JsErrStr(err.Error())
}

func (w *ChessWasm) JsDebugErr(err error) js.Value {
	if w.IsDebug() {
		return w.JsErr(err)
	}
	return js.Undefined()
}

func (w *ChessWasm) JsErrStr(err string) js.Value {
	w.Global.Get("console").Call("error", "an error occurred: "+err)
	return js.Undefined()
}

func (w *ChessWasm) deserializeBoard(value js.Value) (chess.Board, error) {
	input := make([]byte, value.Length())
	js.CopyBytesToGo(input, value)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(input, &pbBoard); err != nil {
		return chess.Board{}, err
	}
	board, err := chess.DeserializeBoard(&pbBoard)
	if err != nil {
		return chess.Board{}, err
	}
	return board, nil
}

func (w *ChessWasm) deserializeGame(value js.Value) (*chess.Game, error) {
	gameBytes := make([]byte, value.Length())
	js.CopyBytesToGo(gameBytes, value)

	var pbGameIn pb.ChessGame
	if err := proto.Unmarshal(gameBytes, &pbGameIn); err != nil {
		return nil, err
	}
	game, err := chess.DeserializeGame(&pbGameIn)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

func (w *ChessWasm) deserializeMove(value js.Value) (chess.Move, error) {
	moveBytes := make([]byte, value.Length())
	js.CopyBytesToGo(moveBytes, value)

	var pbMoveIn pb.Move
	if err := proto.Unmarshal(moveBytes, &pbMoveIn); err != nil {
		return chess.Move{}, err
	}
	pm := chess.DeserializeMove(&pbMoveIn)
	return pm, nil
}

func (w *ChessWasm) deserializeHistMoveList(value js.Value) ([]chess.HistMove, error) {
	moveBytes := make([]byte, value.Length())
	js.CopyBytesToGo(moveBytes, value)

	var pbMoveList pb.HistMoves
	if err := proto.Unmarshal(moveBytes, &pbMoveList); err != nil {
		return nil, err
	}

	return chess.DeserializeHistMoveList(pbMoveList.Moves)
}

func (w *ChessWasm) serializeGame(game *chess.Game) any {
	output, err := proto.Marshal(chess.SerializeGame(game))
	if err != nil {
		return w.JsErr(err)
	}

	out := w.Global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)
	return out
}

func (w *ChessWasm) GetGame(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return w.JsErrStr("fn expects at least 1 args")
	}

	boardUInt8Arr := args[0]

	var board chess.Board
	if boardUInt8Arr.IsUndefined() {
		board = chess.InitialBoard()
	} else {
		b, err := w.deserializeBoard(boardUInt8Arr)
		if err != nil {
			return w.JsErr(err)
		}
		board = b
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	return w.serializeGame(&game)
}

func (w *ChessWasm) MakeMove(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return w.JsErrStr("fn expects at least 2 args")
	}

	gameUInt8Arr := args[0]
	moveUint8Arr := args[1]

	game, err := w.deserializeGame(gameUInt8Arr)
	if err != nil {
		return w.JsErr(err)
	}
	game.EnsurePieceMoves()

	hasMove := !moveUint8Arr.IsUndefined()
	if hasMove {
		move, err := w.deserializeMove(moveUint8Arr)
		if err != nil {
			return w.JsErr(err)
		}
		if _, err := game.MakeValidMove(move); err != nil {
			return w.JsDebugErr(err)
		}
	}

	return w.serializeGame(game)
}

func (w *ChessWasm) FenToGame(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return w.JsErrStr("fn expects at least 1 arg")
	}

	fenString := args[0]

	board, err := chess.ParseFen(fenString.String())
	if err != nil {
		return js.ValueOf([]any{nil, err.Error()})
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	return js.ValueOf([]any{w.serializeGame(&game), ""})
}

func (w *ChessWasm) BoardToFen(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return w.JsErrStr("fn expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	board, err := w.deserializeBoard(inputUInt8Arr)
	if err != nil {
		return w.JsErr(err)
	}

	return js.ValueOf(board.Fen())
}

func (w *ChessWasm) GameAtMoveIndex(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return w.JsErrStr("fn expects at least 2 args")
	}

	boardUInt8Arr := args[0]
	initialBoard, err := w.deserializeBoard(boardUInt8Arr)
	if err != nil {
		return w.JsErr(err)
	}

	movesUInt8Arr := args[1]
	moves, err := w.deserializeHistMoveList(movesUInt8Arr)
	if err != nil {
		return w.JsErr(err)
	}

	moveIndex := args[2]
	if moveIndex.Type() != js.TypeNumber {
		return w.JsErrStr("moveIndex must be a number")
	}
	moveIdx := moveIndex.Int()

	game, err := chess.JumpMoveIndex(initialBoard, moves, moveIdx)
	if err != nil {
		return w.JsErr(err)
	}

	game.InitPieceMoves()
	return w.serializeGame(game)
}

func RegisterChessModule(version string) {
	global := js.Global()
	wasm := &ChessWasm{Global: global, Version: version}
	global.Set("getGame", js.FuncOf(wasm.GetGame))
	global.Set("makeMove", js.FuncOf(wasm.MakeMove))
	global.Set("fenToGame", js.FuncOf(wasm.FenToGame))
	global.Set("boardToFen", js.FuncOf(wasm.BoardToFen))
	global.Set("gameAtMoveIndex", js.FuncOf(wasm.GameAtMoveIndex))
}
