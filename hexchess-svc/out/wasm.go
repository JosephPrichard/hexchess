package out

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"syscall/js"
)

type Wasm struct {
	Global js.Value
}

func (w *Wasm) JsLog(args ...any) {
	w.Global.Get("console").Call("log", args...)
}

func (w *Wasm) JsErr(err error) js.Value {
	return w.JsErrStr(err.Error())
}

func (w *Wasm) JsErrStr(err string) js.Value {
	w.Global.Get("console").Call("error", err)
	return js.Undefined()
}

func (w *Wasm) GetInitialGame(_ js.Value, _ []js.Value) any {
	game := chess.Game{Board: chess.InitialBoard()}
	game.InitPieceMoves()

	output, err := proto.Marshal(chess.SerializeGame(&game))
	if err != nil {
		return w.JsErr(err)
	}

	out := w.Global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)
	return out
}

func (w *Wasm) MakeMove(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return w.JsErrStr("fn expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbMoveIn pb.MakeMoveInput
	if err := proto.Unmarshal(boardIn, &pbMoveIn); err != nil {
		return w.JsErr(err)
	}
	game, err := chess.DeserializeGame(pbMoveIn.Game)
	if err != nil {
		return w.JsErr(err)
	}

	if pbMoveIn.Move != nil {
		pm := chess.DeserializeMove(pbMoveIn.Move)
		if err := game.ValidateMove(pm); err != nil {
			return w.JsErr(err)
		}
		game.MakeMove(chess.Move{From: pm.From, To: pm.To, Promotion: chess.Promotion(pbMoveIn.Move.Promotion)})
		game.InitPieceMoves()
	}

	output, err := proto.Marshal(chess.SerializeGame(&game))
	if err != nil {
		return w.JsErr(err)
	}

	out := w.Global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)
	return out
}

func (w *Wasm) GetMoves(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return w.JsErrStr("fn expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(boardIn, &pbBoard); err != nil {
		return w.JsErr(err)
	}
	board, err := chess.DeserializeBoard(&pbBoard)
	if err != nil {
		return w.JsErr(err)
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	output, err := proto.Marshal(chess.SerializeGame(&game))
	if err != nil {
		return w.JsErr(err)
	}

	out := w.Global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)
	return out
}

func (w *Wasm) FenToGame(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return w.JsErrStr("fn expects at least 1 arg")
	}

	fenString := args[0]

	board, err := chess.ParseFen(fenString.String())
	if err != nil {
		return js.ValueOf([]any{nil, err.Error()})
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	output, err := proto.Marshal(chess.SerializeGame(&game))
	if err != nil {
		return w.JsErr(err)
	}

	out := w.Global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)
	return js.ValueOf([]any{out, ""})
}

func (w *Wasm) BoardToFen(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return w.JsErrStr("fn expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(input, &pbBoard); err != nil {
		return w.JsErr(err)
	}
	board, err := chess.DeserializeBoard(&pbBoard)
	if err != nil {
		return w.JsErr(err)
	}
	return js.ValueOf(board.Fen())
}

func (w *Wasm) GetMoveNotations(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return w.JsErrStr("fn expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbHistMoves pb.HistMoves
	if err := proto.Unmarshal(input, &pbHistMoves); err != nil {
		return w.JsErr(err)
	}

	moves := make([]any, 0, len(pbHistMoves.Moves))
	for _, pbHm := range pbHistMoves.Moves {
		hm := chess.DeserializeHistMove(pbHm)
		moves = append(moves, hm.String())
	}
	return js.ValueOf(moves)
}

func RegisterWasmModule(global js.Value) {
	wasm := &Wasm{Global: global}
	global.Set("getInitialGame", js.FuncOf(wasm.GetInitialGame))
	global.Set("makeMove", js.FuncOf(wasm.MakeMove))
	global.Set("getMoves", js.FuncOf(wasm.GetMoves))
	global.Set("fenToGame", js.FuncOf(wasm.FenToGame))
	global.Set("boardToFen", js.FuncOf(wasm.BoardToFen))
	global.Set("getMoveNotations", js.FuncOf(wasm.GetMoveNotations))
}
