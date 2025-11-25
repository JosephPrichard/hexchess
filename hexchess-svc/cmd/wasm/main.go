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

func jsErr(err string) js.Value {
	global.Get("console").Call("error", err)
	return js.Undefined()
}

func GetInitialGame(_ js.Value, _ []js.Value) interface{} {
	game := chess.Game{Board: chess.InitialBoard()}
	game.InitPieceMoves()

	pbGame, err := chess.MapPbGame(game)
	if err != nil {
		return jsErr(err.Error())
	}
	output, err := proto.Marshal(pbGame)
	if err != nil {
		return jsErr(err.Error())
	}
	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func makeMoveErr(violation chess.MoveViolation) js.Value {
	switch violation {
	case chess.ViolatesOutOfBounds:
		return jsErr("violation: Out of bounds move")
	case chess.ViolatesNoop:
		return jsErr("violation: Noop move")
	case chess.ViolatesIllegalMove:
		return jsErr("violation: Illegal move")
	default:
		return jsErr("violation: Unknown violation")
	}
}

func MakeMove(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErr("makeMove expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbMoveIn pb.MakeMoveInput
	if err := proto.Unmarshal(boardIn, &pbMoveIn); err != nil {
		return jsErr(err.Error())
	}
	game, err := chess.MapGame(pbMoveIn.Game)
	if err != nil {
		return jsErr(err.Error())
	}
	pm := chess.MapPieceMove(pbMoveIn.Move)

	if pbMoveIn.Move != nil {
		if violation := game.ValidateMove(pm, pbMoveIn.Validate); violation != chess.ViolatesNone {
			return makeMoveErr(violation)
		}
		game.MakeMove(pm.From, pm.To)
		game.InitPieceMoves()
	}

	pbGameOut, err := chess.MapPbGame(game)
	if err != nil {
		return jsErr(err.Error())
	}
	output, err := proto.Marshal(pbGameOut)
	if err != nil {
		return jsErr(err.Error())
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func GetMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErr("getMoves expects at least 1 args")
	}

	inputUInt8Arr := args[0]
	boardIn := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(boardIn, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(boardIn, &pbBoard); err != nil {
		return jsErr(err.Error())
	}
	board, err := chess.MapBoard(&pbBoard)
	if err != nil {
		return jsErr(err.Error())
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	pbGameOut, err := chess.MapPbGame(game)
	if err != nil {
		return jsErr(err.Error())
	}
	output, err := proto.Marshal(pbGameOut)
	if err != nil {
		return jsErr(err.Error())
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	return out
}

func FenToGame(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErr("fenToBoard expects at least 1 arg")
	}

	fenString := args[0]
	fen := fenString.String()

	board, err := chess.ParseFen(fen)
	if err != nil {
		obj := global.Get("Object").New()
		obj.Set("err", err.Error())
		return obj
	}

	game := chess.Game{Board: board}
	game.InitPieceMoves()

	pbGame, err := chess.MapPbGame(game)
	if err != nil {
		return jsErr(err.Error())
	}
	output, err := proto.Marshal(pbGame)
	if err != nil {
		return jsErr(err.Error())
	}

	out := global.Get("Uint8Array").New(len(output))
	js.CopyBytesToJS(out, output)

	obj := global.Get("Object").New()
	obj.Set("game", out)

	return obj
}

func BoardToFen(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErr("boardToFen expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbBoard pb.ChessBoard
	if err := proto.Unmarshal(input, &pbBoard); err != nil {
		return jsErr(err.Error())
	}
	board, err := chess.MapBoard(&pbBoard)
	if err != nil {
		return jsErr(err.Error())
	}

	fen := board.Fen()
	fenValue := js.ValueOf(fen)
	return fenValue
}

func GetMoveNotations(_ js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		return jsErr("getMoveNotations expects at least 1 arg")
	}

	inputUInt8Arr := args[0]
	input := make([]byte, inputUInt8Arr.Length())
	js.CopyBytesToGo(input, inputUInt8Arr)

	var pbHistMoves pb.HistMoves
	if err := proto.Unmarshal(input, &pbHistMoves); err != nil {
		return jsErr(err.Error())
	}

	moves := make([]any, 0, len(pbHistMoves.Moves))
	for _, pbHm := range pbHistMoves.Moves {
		hm := chess.MapHistMove(pbHm)
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
