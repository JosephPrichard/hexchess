package main

import (
	"fmt"
	"math/rand/v2"
	"xk6-hexchess/pb"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modules"
	"google.golang.org/protobuf/proto"
)

const wsHelperExtname = "k6/x/hexchess/websocket"

func init() {
	modules.Register(wsHelperExtname, &WSHelperRoot{})

	fmt.Printf("registered xk6 extension: %s\n", wsHelperExtname)
}

type WSHelperRoot struct{}

type WSHelperInstance struct {
	vu modules.VU
}

var (
	_ modules.Module   = &WSHelperRoot{}
	_ modules.Instance = &WSHelperInstance{}
)

func (r *WSHelperRoot) NewModuleInstance(vu modules.VU) modules.Instance {
	return &WSHelperInstance{vu: vu}
}

func (m *WSHelperInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"handleGameInputEvent": m.HandleGameInputEvent,
		},
	}
}

type GameInputReq struct {
	RecvBytes      []byte `js:"recvBytes"`
	InputMessageID string `js:"inputMessageId"`
}

type GameInputResp struct {
	NextInputBytes []byte
	NextInputType  string
	PrevMessageID  string
	PrevType       string
	IsTerminal     bool
}

func sobekError(rt *sobek.Runtime, err error) *sobek.Object {
	o := rt.NewObject()
	o.Set("error", fmt.Sprintf("%v", err))
	return o
}

func sobekNextInputResp(rt *sobek.Runtime, r GameInputResp) *sobek.Object {
	o := rt.NewObject()
	if r.NextInputBytes != nil {
		o.Set("nextInputBytes", rt.NewArrayBuffer(r.NextInputBytes))
	}
	o.Set("prevMessageId", r.PrevMessageID)
	o.Set("prevType", r.PrevType)
	o.Set("isTerminal", r.IsTerminal)
	return o
}

func (m *WSHelperInstance) HandleGameInputEvent(req GameInputReq) (o *sobek.Object) {
	runtime := m.vu.Runtime()

	defer func() {
		if err := recover(); err != nil {
			o = runtime.NewObject()
			o.Set("error", fmt.Sprintf("%v", err))
		}
	}()

	var output pb.GameOutput
	if err := proto.Unmarshal(req.RecvBytes, &output); err != nil {
		return sobekError(runtime, err)
	}
	outputType := stringOfOutput(&output)

	var game *pb.ChessGame
	switch v := output.Value.(type) {
	case *pb.GameOutput_Move:
		game = v.Move.Game
	case *pb.GameOutput_Init:
		game = v.Init.State.Game
	case *pb.GameOutput_Replay:
		return sobekNextInputResp(runtime, GameInputResp{IsTerminal: true})
	default:
		return sobekNextInputResp(runtime, GameInputResp{PrevType: outputType, PrevMessageID: output.MessageId})
	}
	
	var pieceMovesList []*pb.PieceMoves
	if game.Board.IsWhiteTurn {
		pieceMovesList = game.WhiteMoves
	} else {
		pieceMovesList = game.BlackMoves
	}

	type movePair struct {
		fromFile int32
		fromRank int32
		toFile   int32
		toRank   int32
	}
	var moves []movePair

	// flatten piece moves to select a random move and terminate if no moves are available
	for _, pieceMoves := range pieceMovesList {
		for _, moveToInt64 := range pieceMoves.Moves {
			toFile := int32(moveToInt64 & 0xFFFFFFFF)
			toRank := int32(moveToInt64 >> 32)

			moves = append(moves, movePair{
				fromFile: pieceMoves.FromFile,
				fromRank: pieceMoves.FromRank,
				toFile:   toFile,
				toRank:   toRank,
			})
		}
	}
	if len(moves) == 0 {
		return sobekNextInputResp(runtime, GameInputResp{PrevType: outputType, PrevMessageID: output.MessageId, IsTerminal: true})
	}

	move := moves[rand.IntN(len(moves))]

	input := &pb.GameInput{
		MessageId: req.InputMessageID,
		Value:     &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{
			Promotion: 1, // send Queen, in case promotion is required.
			FromFile:  move.fromFile,
			FromRank:  move.fromRank,
			ToFile:    move.toFile,
			ToRank:    move.toRank,
		}}},
	}
	inputType := stringOfInput(input)

	inputBytes, err := proto.Marshal(input)
	if err != nil {
		return sobekError(runtime, err)
	}

	return sobekNextInputResp(runtime, GameInputResp{
		NextInputBytes: inputBytes,
		NextInputType:  inputType,
		PrevType:       outputType,
		PrevMessageID:  output.MessageId,
	})
}