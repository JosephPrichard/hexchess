package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"
	"xk6-hexchess/pb"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
			"runGameSockets": RunGameSockets,
		},
	}
}

type RunGameSocketsOpts struct {
	TargetEndpoint string `js:"targetEndpoint"`
	GameID         string `js:"gameId"`

	StaggerMs   int64 `js:"staggerMs"`
	TimeoutSecs int64 `js:"timeoutSecs"`
	MaxMoves    int64 `js:"maxMoves"`
}

type RunGameSocketsReq struct {
	SessionIDs []string `js:"sessionIds"`
	RunGameSocketsOpts
}

type RunGameSocketsResp struct {
	Metrics map[string]Metric `js:"metrics"`
	Error   error             `js:"error"`
}

func RunGameSockets(req RunGameSocketsReq) RunGameSocketsResp {
	slog.Info("running game sockets", "gameID", req.GameID, "sessionIDs", req.SessionIDs)

	metrics := NewMetricMap()

	var respChans []chan error
	var errs []error

	for _, sessionID := range req.SessionIDs {
		respChan := make(chan error, 1)
		go runGameSocketAsync(metrics, respChan, RunGameSocketReq{
			SessionID:          sessionID,
			RunGameSocketsOpts: req.RunGameSocketsOpts,
		})
		respChans = append(respChans, respChan)
	}

	for _, ch := range respChans {
		errs = append(errs, <-ch)
	}

	return RunGameSocketsResp{Metrics: metrics.MetricMap, Error: errors.Join(errs...)}
}

type GameSocketState struct {
	IsSendReady  bool
	MessageCount int64
}

type RunGameSocketReq struct {
	SessionID string
	RunGameSocketsOpts
}

func runGameSocketAsync(metrics *Metrics, respChan chan error, req RunGameSocketReq) {
	err := runGameSocket(metrics, req)
	if errors.Is(err, context.Canceled) {
		slog.Warn("game websocket timed out", "request", req)
		err = nil // do not propagate error to VU.
	}
	respChan <- err
}

func runGameSocket(metrics *Metrics, req RunGameSocketReq) error {
	slog.Info("opening game socket", "request", req)

	ctx := context.Background()
	cancel := func() {}
	if req.TimeoutSecs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(req.TimeoutSecs))
	}
	defer cancel()

	params := url.Values{}
	params.Set("gameId", req.GameID)
	params.Set("sessionId", req.SessionID)

	wsURL := fmt.Sprintf("%s/api/ws/game?%s", req.TargetEndpoint, params.Encode())

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, http.Header{})
	if err != nil {
		return fmt.Errorf("failed to dial websocket %s: %w", wsURL, err)
	}
	defer conn.Close()

	openMessageID := uuid.NewString()
	metrics.NewMetric(openMessageID, "open")

	var socketState GameSocketState

	type Input struct {
		bytes     []byte
		messageID string
		inputType string
	}
	var inputBuf []Input

	for {
		// read phase: read outputs and update recv loop state
		_, recvBytes, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read message from websocket %s: %w", wsURL, err)
		}

		// unmarshal phase: extract outputs and evaluate stop condition
		output, err := UnmarshalGameOutput(recvBytes, req.GameID, req.SessionID)
		if err != nil {
			return err
		}
		if output.RecvType == "init" {
			metrics.AppendMetric(openMessageID, "init")
		}
		if output.RecvMessageID != "" {
			metrics.AppendMetric(output.RecvMessageID, output.RecvType)
		}
		if output.IsTerminal {
			return nil
		}

		// check: update socket state based on new received outputs
		if output.Players != nil {
			if output.Players.BlackPlayer != nil && output.Players.WhitePlayer != nil {
				socketState.IsSendReady = true
			}
		}
		socketState.MessageCount++
		if socketState.MessageCount > req.MaxMoves {
			return nil
		}

		// marshal phase: buffer inputs to be sent and evaluate stop condition
		inputMessageID := uuid.NewString()
		input, err := MarshalNextGameInput(output.Game, inputMessageID)
		if err != nil {
			return err
		}
		if input.InputBytes != nil {
			inputBuf = append(inputBuf, Input{bytes: input.InputBytes, messageID: inputMessageID, inputType: input.InputType})
		}
		if input.IsTerminal {
			return nil
		}

		// write phase: send buffered inputs if send is legal
		if !socketState.IsSendReady {
			continue
		}
		if req.StaggerMs > 0 {
			time.Sleep(time.Duration(req.StaggerMs) * time.Millisecond)
		}

		for _, input := range inputBuf {
			if err := conn.WriteMessage(websocket.BinaryMessage, input.bytes); err != nil {
				slog.Error("failed to write message to websocket", "error", err)
			}
			metrics.NewMetric(input.messageID, input.inputType)
		}
		inputBuf = inputBuf[:0]
	}
}

type GameOutput struct {
	Game    *pb.ChessGame
	Players *pb.PlayersOutput

	RecvMessageID string
	RecvType      string

	IsTerminal bool
}

func UnmarshalGameOutput(outputBytes []byte, gameID string, sessionID string) (GameOutput, error) {
	var output pb.GameOutput
	if err := proto.Unmarshal(outputBytes, &output); err != nil {
		return GameOutput{}, err
	}
	outputType := stringOfOutput(&output)

	var game *pb.ChessGame
	var players *pb.PlayersOutput
	var isTerminal bool

	switch v := output.Value.(type) {
	case *pb.GameOutput_Move:
		game = v.Move.Game
	case *pb.GameOutput_Init:
		game = v.Init.State.Game
		slog.Debug("received init event", "gameID", gameID, "sessionID", sessionID)
	case *pb.GameOutput_Replay:
		isTerminal = true
	case *pb.GameOutput_Players:
		players = v.Players
		slog.Debug("received players event", "gameID", gameID, "sessionID", sessionID, "players", v.Players)
	}

	return GameOutput{
		Game:       game,
		Players:    players,
		IsTerminal: isTerminal,

		RecvType:      outputType,
		RecvMessageID: output.MessageId,
	}, nil
}

type GameInput struct {
	InputBytes []byte
	InputType  string

	IsTerminal bool
}

func MarshalNextGameInput(game *pb.ChessGame, inputMessageID string) (GameInput, error) {
	if game == nil || game.Board == nil {
		return GameInput{}, nil
	}

	var pieceMovesList []*pb.PieceMoves
	if game.Board.IsWhiteTurn {
		pieceMovesList = game.WhiteMoves
	} else {
		pieceMovesList = game.BlackMoves
	}

	type Move struct {
		fromFile uint32
		fromRank uint32
		toFile   uint32
		toRank   uint32
	}
	var moves []Move

	// flatten piece moves to select a random move and terminate if no moves are available
	for _, pieceMoves := range pieceMovesList {
		for _, hex := range pieceMoves.Moves {
			moves = append(moves, Move{
				fromFile: pieceMoves.FromFile,
				fromRank: pieceMoves.FromRank,
				toFile:   hex.File,
				toRank:   hex.Rank,
			})
		}
	}
	if len(moves) == 0 {
		return GameInput{IsTerminal: true}, nil
	}

	move := moves[rand.IntN(len(moves))]

	input := &pb.GameInput{
		MessageId: inputMessageID,
		Value: &pb.GameInput_Move{Move: &pb.MoveInput{Move: &pb.Move{
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
		return GameInput{}, err
	}

	slog.Debug("producing game input", "inputMessageID", inputMessageID, "inputType", inputType)

	return GameInput{InputBytes: inputBytes, InputType: inputType}, nil
}
