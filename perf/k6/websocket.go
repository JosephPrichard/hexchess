package main

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"xk6-hexchess/pb"

	"github.com/google/uuid"
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
			"createChatGameInput":  m.CreateChatGameInput,
		},
	}
}

type GameInputReq struct {
	RecvBytes    []byte   `js:"recvBytes"`
	InputMessageID string `js:"inputMessageId"`
}

type ChatGameReq struct {
	RecvBytes    []byte   `js:"recvBytes"`
	InputMessageID string `js:"inputMessageId"`
}

type GameInputResp struct {
	NextInputBytes      []byte
	NextInputType           string
	PrevMessageID string
	PrevType      string
	IsTerminal          bool
}

func MarshalNextInputResp(rt *sobek.Runtime, r GameInputResp) *sobek.Object {
	o := rt.NewObject()
	if r.NextInputBytes != nil {
		o.Set("nextInputBytes", rt.NewArrayBuffer(r.NextInputBytes))
	}
	o.Set("prevMessageId", r.PrevMessageID)
	o.Set("prevType", r.PrevType)
	o.Set("isTerminal", r.IsTerminal)
	return o
}

func (m *WSHelperInstance) CreateChatGameInput() (o *sobek.Object) {
	runtime := m.vu.Runtime()
	o = runtime.NewObject()

	defer func() {
		if err := recover(); err != nil {
			slog.Error("failed to create chat input", "error", err)
		}
	}()

	messageID := uuid.NewString()

	bytes, err := proto.Marshal(&pb.GameInput{
		MessageId: messageID,
		Value:     &pb.GameInput_Chat{Chat: &pb.ChatInput{Message: "Hello from k6!"}},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to marshal chat input: %+v", err))
	}

	o.Set("chatMessageId", messageID)
	o.Set("chatInputBytes", runtime.NewArrayBuffer(bytes))

	return o
}

func (m *WSHelperInstance) HandleGameInputEvent(req GameInputReq) (o *sobek.Object) {
	runtime := m.vu.Runtime()

	defer func() {
		if err := recover(); err != nil {
			slog.Error("failed to make next game input", "error", err)
			o = runtime.NewObject()
		}
	}()

	var output pb.GameOutput
	if err := proto.Unmarshal(req.RecvBytes, &output); err != nil {
		panic(fmt.Sprintf("failed to unmarshal chess state: %+v", err))
	}
	outputType := stringOfOutput(&output)

	var game *pb.ChessGame

	switch v := output.Value.(type) {
	case *pb.GameOutput_Move:
		game = v.Move.Game
	case *pb.GameOutput_Init:
		game = v.Init.State.Game
	case *pb.GameOutput_Replay:
		return MarshalNextInputResp(runtime, GameInputResp{IsTerminal: true})
	default:
		return MarshalNextInputResp(runtime, GameInputResp{PrevType: outputType, PrevMessageID: output.MessageId})
	}

	move := pickChessStateMove(game)
	if move == nil {
		return MarshalNextInputResp(runtime, GameInputResp{
			PrevType:      outputType,
			PrevMessageID: output.MessageId,
			IsTerminal: true,
		})
	}

	input := &pb.GameInput{
		MessageId: req.InputMessageID, 
		Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: move}},
	}
	inputType := stringOfInput(input)

	inputBytes, err := proto.Marshal(input)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal move %+v", err))
	}

	return MarshalNextInputResp(runtime, GameInputResp{
		NextInputBytes:      inputBytes,
		NextInputType:           inputType,
		PrevType:      outputType,
		PrevMessageID: output.MessageId,
	})
}

func pickChessStateMove(game *pb.ChessGame) *pb.Move {
	// precondition: pbChessState field tree is fully initialized.
	var gameMoves []*pb.PieceMoves
	if game.Board.IsWhiteTurn {
		gameMoves = game.WhiteMoves
	} else {
		gameMoves = game.BlackMoves
	}

	type Move struct {
		FromFile int32
		FromRank int32
		ToFile int32
		ToRank int32
	}
	var moves []Move

	for _, pieceMoves := range gameMoves {
		for _, moveToInt64 := range pieceMoves.Moves {
			toFile := int32(moveToInt64 & 0xFFFFFFFF)
			toRank := int32(moveToInt64 >> 32)

			moves = append(moves, Move{
				FromFile: pieceMoves.FromFile,
				FromRank: pieceMoves.FromRank, 
				ToFile: toFile, 
				ToRank: toRank,
			})
		}
	}
	if len(moves) == 0 {
		return nil
	}

	move := moves[rand.IntN(len(moves))]
	return &pb.Move{
		Promotion: 1, // Queen, if promotion is required.
		FromFile: move.FromFile, 
		FromRank: move.FromRank, 
		ToFile: move.ToFile, 
		ToRank: move.ToRank,
	}
}