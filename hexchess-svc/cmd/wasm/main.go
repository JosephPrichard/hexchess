package main

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"syscall/js"
)

var global = js.Global()

func jsLog(args ...any) {
	global.Get("console").Call("log", args...)
}

func jsErr(err error) js.Value {
	return jsErrStr(err.Error())
}

func jsErrStr(err string) js.Value {
	global.Get("console").Call("error", err)
	return js.Undefined()
}

func GetInitialGame(_ js.Value, _ []js.Value) interface{} {
	game := chess.Game{Board: chess.InitialBoard()}
	game.InitPieceMoves()

	pbGame, err := chess.SerializeGame(game)
	if err != nil {
		return jsErr(err)
	}
	output, err := proto.Marshal(pbGame)
	if err != nil {
		return jsErr(err)
	}
	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func MakeMove(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErrStr("fn expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbMoveIn pb.MakeMoveInput
	if err := proto.Unmarshal(boardIn, &pbMoveIn); err != nil {
		return jsErr(err)
	}
	game, err := chess.DeserializeGame(pbMoveIn.Game)
	if err != nil {
		return jsErr(err)
	}
	pm := chess.DeserializeMove(pbMoveIn.Move)

	if pbMoveIn.Move != nil {
		if err := game.ValidateMove(pm); err != nil {
			return jsErr(err)
		}
		game.MakeMove(chess.Move{From: pm.From, To: pm.To})
		game.InitPieceMoves()
	}

	pbGameOut, err := chess.SerializeGame(game)
	if err != nil {
		return jsErr(err)
	}
	output, err := proto.Marshal(pbGameOut)
	if err != nil {
		return jsErr(err)
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func GetMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErrStr("fn expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(boardIn, &pbBoard); err != nil {
		return jsErr(err)
	}
	board, err := chess.DeserializeBoard(&pbBoard)
	if err != nil {
		return jsErr(err)
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	pbGameOut, err := chess.SerializeGame(game)
	if err != nil {
		return jsErr(err)
	}
	output, err := proto.Marshal(pbGameOut)
	if err != nil {
		return jsErr(err)
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func FenToGame(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErrStr("fn expects at least 1 arg")
	}

	fenString := args[0]
	fen := fenString.String()

	board, err := chess.ParseFen(fen)
	if err != nil {
		obj := global.Get("Object").New()
		obj.Set("err", err)
		return obj
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	pbGame, err := chess.SerializeGame(game)
	if err != nil {
		return jsErr(err)
	}
	output, err := proto.Marshal(pbGame)
	if err != nil {
		return jsErr(err)
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	obj := global.Get("Object").New()
	obj.Set("game", out)

	return obj
}

func BoardToFen(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErrStr("fn expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(input, &pbBoard); err != nil {
		return jsErr(err)
	}
	board, err := chess.DeserializeBoard(&pbBoard)
	if err != nil {
		return jsErr(err)
	}

	fen := board.Fen()
	fenValue := js.ValueOf(fen)
	return fenValue
}

func GetMoveNotations(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErrStr("fn expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbHistMoves pb.HistMoves
	if err := proto.Unmarshal(input, &pbHistMoves); err != nil {
		return jsErr(err)
	}

	moves := make([]any, 0, len(pbHistMoves.Moves))
	for _, pbHm := range pbHistMoves.Moves {
		hm := chess.DeserializeHistMove(pbHm)
		moves = append(moves, hm.String())
	}

	notationsValue := js.ValueOf(moves)
	return notationsValue
}

func main() {
	jsLog("Begin initializing wasm module")

	global.Set("getInitialGame", js.FuncOf(GetInitialGame))
	global.Set("makeMove", js.FuncOf(MakeMove))
	global.Set("getMoves", js.FuncOf(GetMoves))
	global.Set("fenToGame", js.FuncOf(FenToGame))
	global.Set("boardToFen", js.FuncOf(BoardToFen))
	global.Set("getMoveNotations", js.FuncOf(GetMoveNotations))

	jsLog("Finished initializing wasm module")

	select {}
}
