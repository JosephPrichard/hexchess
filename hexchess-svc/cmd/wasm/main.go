package main

import (
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"syscall/js"
)

func handleJsErr(err string) js.Value {
	js.Global().Get("console").Call("error", err)
	return js.Undefined()
}

func GetInitialBoard(_ js.Value, _ []js.Value) interface{} {
	js.Global().Get("console").Call("log", "GetInitialBoard called")

	pbBoard, err := chess.MapPbBoard(chess.InitialBoard())
	if err != nil {
		return handleJsErr(err.Error())
	}
	output, err := proto.Marshal(pbBoard)
	if err != nil {
		return handleJsErr(err.Error())
	}
	out := js.Global().Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	js.Global().Get("console").Call("log", "GetInitialBoard finished")

	return out
}

func MakeBoardMove(_ js.Value, args []js.Value) interface{} {
	js.Global().Get("console").Call("log", "MakeBoardMove called")

	if len(args) == 0 {
		return handleJsErr("makeBoardMove expects at least 2 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbMoveIn pb.MakeMoveInput
	if err := proto.Unmarshal(boardIn, &pbMoveIn); err != nil {
		return handleJsErr(err.Error())
	}

	board, err := chess.MapBoard(pbMoveIn.Board)
	if err != nil {
		return handleJsErr(err.Error())
	}
	var pm chess.PieceMove
	var hasMove bool
	if pbMoveIn.Move != nil {
		pm = chess.MapPieceMove(pbMoveIn.Move)
		hasMove = true
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()
	if hasMove {
		game.MakeMove(pm.From, pm.To)
	}

	pbGameOut, err := chess.MapPbGame(game)
	if err != nil {
		return handleJsErr(err.Error())
	}
	output, err := proto.Marshal(pbGameOut)
	if err != nil {
		return handleJsErr(err.Error())
	}

	out := js.Global().Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	js.Global().Get("console").Call("log", "MakeBoardMove finished")

	return out
}

func FenToGame(_ js.Value, args []js.Value) interface{} {
	js.Global().Get("console").Call("log", "FenToGame called")

	if len(args) == 0 {
		return handleJsErr("fenToBoard expects at least 1 arg")
	}

	fenString := args[0]
	fen := fenString.String()

	board, err := chess.ParseFen(fen)
	if err != nil {
		obj := js.Global().Get("Object").New()
		obj.Set("err", err.Error())
		return obj
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	pbGame, err := chess.MapPbGame(game)
	if err != nil {
		return handleJsErr(err.Error())
	}
	output, err := proto.Marshal(pbGame)
	if err != nil {
		return handleJsErr(err.Error())
	}

	js.Global().Get("console").Call("log", "FenToGame finished")

	out := js.Global().Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	obj := js.Global().Get("Object").New()
	obj.Set("game", out)

	js.Global().Get("console").Call("log", "FenToGame returning")

	return obj
}

func BoardToFen(_ js.Value, args []js.Value) interface{} {
	js.Global().Get("console").Call("log", "BoardToFen called")

	if len(args) == 0 {
		return handleJsErr("boardToFen expects at least 1 arg")
	}

	boardUInt8Arr := args[0]
	input := make([]byte, boardUInt8Arr.Length())
	js.CopyBytesToGo(input, boardUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(input, &pbBoard); err != nil {
		return handleJsErr(err.Error())
	}
	board, err := chess.MapBoard(&pbBoard)
	if err != nil {
		return handleJsErr(err.Error())
	}

	fen := board.Fen()
	fenValue := js.ValueOf(fen)

	js.Global().Get("console").Call("log", "BoardToFen finished")

	return fenValue
}

func main() {
	js.Global().Get("console").Call("log", "Begin initializing wasm module")

	js.Global().Set("getInitialBoard", js.FuncOf(GetInitialBoard))
	js.Global().Set("makeMove", js.FuncOf(MakeBoardMove))
	js.Global().Set("fenToGame", js.FuncOf(FenToGame))
	js.Global().Set("boardToFen", js.FuncOf(BoardToFen))

	c := make(chan struct{})
	<-c
}
