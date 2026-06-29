package websocket

import (
	"fmt"
	"log/slog"
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
			"nextGameInput": m.NextGameInput,
		},
	}
}

type NextInputReq struct {
	PrevOutputBytes    []byte `js:"prevOutputBytes"`
	NextInputMessageID string `js:"nextInputMessageId"`
}

type NextInputResp struct {
	NextInputBytes      []byte `js:"nextInputBytes"`
	PrevOutputMessageID string `js:"prevOutputMessageId"`
	PrevOutputType      string `js:"prevOutputType"`
	IsTerminal          bool   `js:"isTerminal"`
}

//goland:noinspection ALL
func MakeNextInputResp(rt *sobek.Runtime, o NextInputResp) *sobek.Object {
	obj := rt.NewObject()
	if o.NextInputBytes != nil {
		obj.Set("nextInputBytes", rt.NewArrayBuffer(o.NextInputBytes))
	}
	obj.Set("prevOutputMessageId", o.PrevOutputMessageID)
	obj.Set("prevOutputType", o.PrevOutputType)
	obj.Set("isTerminal", o.IsTerminal)
	return obj
}

func KindOfOutput(o *pb.GameOutput) string {
	switch o.Value.(type) {
	case *pb.GameOutput_Move:
		return "move"
	case *pb.GameOutput_Error:
		return "error"
	case *pb.GameOutput_Chat:
		return "chat"
	case *pb.GameOutput_Init:
		return "init"
	default:
		return ""
	}
}

func (m *WSHelperInstance) NextGameInput(req NextInputReq) *sobek.Object {
	defer func() {
		if err := recover(); err != nil {
			// prevents taking down the entire vu and provides logging to catch the error.
			slog.Error("failed to produce next game input", "error", err)
		}
	}()

	var output pb.GameOutput
	if err := proto.Unmarshal(req.PrevOutputBytes, &output); err != nil {
		panic(fmt.Sprintf("failed to unmarshal chess state: %+v", err))
	}
	outputType := KindOfOutput(&output)

	var game *pb.ChessGame

	switch v := output.Value.(type) {
	case *pb.GameOutput_Move:
		game = v.Move.Game
	case *pb.GameOutput_Init:
		game = v.Init.State.Game
	case *pb.GameOutput_Replay:
		return MakeNextInputResp(m.vu.Runtime(), NextInputResp{IsTerminal: true})
	default:
		return MakeNextInputResp(m.vu.Runtime(), NextInputResp{})
	}

	// precondition: pbChessState field tree is fully initialized.
	var moves []*pb.PieceMoves
	if game.Board.IsWhiteTurn {
		moves = game.WhiteMoves
	} else {
		moves = game.BlackMoves
	}

	if len(moves) == 0 {
		return MakeNextInputResp(m.vu.Runtime(), NextInputResp{PrevOutputMessageID: output.MessageId, PrevOutputType: outputType, IsTerminal: true})
	}
	pieceMoves := moves[0]
	moveTos := pieceMoves.Moves

	if len(moveTos) == 0 {
		return MakeNextInputResp(m.vu.Runtime(), NextInputResp{PrevOutputMessageID: output.MessageId, PrevOutputType: outputType, IsTerminal: true})
	}
	moveToInt64 := moveTos[0]
	toFile := int32(moveToInt64 & 0xFFFFFFFF)
	toRank := int32(moveToInt64 >> 32)

	nextInput := &pb.GameInput{
		MessageId: req.NextInputMessageID,
		Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{
			FromFile: pieceMoves.FromFile,
			FromRank: pieceMoves.FromRank,
			ToFile:   toFile,
			ToRank:   toRank,
		}}},
	}
	inputBytes, err := proto.Marshal(nextInput)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal move %+v", err))
	}

	return MakeNextInputResp(m.vu.Runtime(), NextInputResp{
		NextInputBytes:      inputBytes,
		PrevOutputType:      outputType,
		PrevOutputMessageID: output.MessageId,
	})
}
