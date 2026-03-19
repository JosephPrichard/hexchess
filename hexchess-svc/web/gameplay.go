package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"
	"hexchess-svc/pb"
	svc "hexchess-svc/services"

	"github.com/gorilla/websocket"
)

type GameSocketContext struct {
	Context context.Context
	GameID  string
	Player  svc.PlayerState
	ErrChan chan error
}

// GameplayChanBufCap start dropping messages when a websocket is behind by this many messages
const GameplayChanBufCap = 10

func (server *Server) HandleGameWs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := query.Get("gameId")
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "err", err)
		return
	}
	defer conn.Close()

	// subscribe before we begin init, so the number of messages we expect as a result of the initialization stage is deterministic.
	subscriber := make(chan message, GameplayChanBufCap)
	server.Broadcasters.GamesCaster.Subscribe(gameID, subscriber)
	defer server.Broadcasters.GamesCaster.Unsubscribe(gameID, subscriber)

	errChan := make(chan error)

	go func() {
		// write back broadcasts (from subscriber) and errors (from input messages) back to the client
	RecvLoop:
		for {
			select {
			case err := <-errChan:
				writeGameMsgErr(ctx, conn, gameID, err)
			case v, ok := <-subscriber:
				if !ok {
					break RecvLoop
				}
				writeMessage(ctx, conn, v)
			}
		}
		conn.Close()
		slog.InfoContext(ctx, "gameplay websocket writer closed", "gameID", gameID)
	}()

	// begin the init phase, which retrieves state and writes back to clients
	player, err := server.handleGameInit(ctx, gameID, sessionID, conn)
	if err != nil {
		// write an error and close if we run into any issues. init is idempotent so the client can retry until everything works
		writeGameInitErr(ctx, conn, gameID, err)
		return
	}

	// read and handle each input message, with each handler running concurrently
	gameSocketCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: player, ErrChan: errChan}
	for {
		_, input, err := conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "failed to read ws message", "err", err)
			break
		}
		go server.handleGameMessage(gameSocketCtx, input)
	}

	slog.InfoContext(ctx, "gameplay websocket reader closed", "gameID", gameID)
}

func isType[T error](err error) bool {
	var t T
	return errors.As(err, &t)
}

func writeGameMsgErr(ctx context.Context, conn *websocket.Conn, gameID string, err error) {
	wsErr := ErrWsFatal
	switch {
	case isType[svc.ErrFinishedGame](err):
		wsErr = ErrWsFinishedGame
	case errors.Is(err, svc.ErrForfeitPlayer):
		wsErr = ErrWsForfeitPlayer
	case isType[svc.ErrStartedGame](err):
		wsErr = ErrWsStartedGame
	case isType[svc.ErrTurn](err):
		wsErr = ErrWsTurn
	case isType[svc.ErrInvalidMove](err):
		wsErr = ErrWsInvalidMove
	case errors.Is(err, svc.ErrNoChessState):
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, svc.ErrUndoCurrPlayer):
		wsErr = ErrWsUndoCurrPlayer
	case errors.Is(err, svc.ErrNoMoveUndo), errors.Is(err, svc.ErrUndoNoop), errors.Is(err, svc.ErrNoUndo):
		wsErr = ErrWsUndoAction
	}

	slog.WarnContext(ctx, "failed to handle ws message", "err", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(SerializeGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal err output", "err", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func writeGameInitErr(ctx context.Context, conn *websocket.Conn, gameID string, err error) {
	var wsErr error
	switch {
	case errors.Is(err, svc.ErrNoChessState):
		wsErr = ErrWsInvalidGame
	default:
		wsErr = ErrWsFatal
	}
	slog.WarnContext(ctx, "failed to initialize gameplay websocket", "err", err, "wsErr", wsErr)

	bytes, err := proto.Marshal(SerializeGameOutputError(gameID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal init err output", "err", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func (server *Server) handleGameInit(ctx context.Context, gameID string, sessionID string, conn *websocket.Conn) (player svc.PlayerState, err error) {
	// apply state updates for the init phase
	player, err = server.Services.GetSession(ctx, sessionID)
	if err != nil {
		return player, fmt.Errorf("get session in game init phase: %w", err)
	}
	s, err := server.Services.JoinGame(ctx, gameID, player)
	if err != nil {
		return player, fmt.Errorf("join game in init game phase: %w", err)
	}

	// produce messages for init phase
	initBytes, err := proto.Marshal(SerializeGameOutputInit(
		gameID,
		svc.SerializeChessState(s),
		svc.SerializePlayer(player),
	))
	if err != nil {
		return player, fmt.Errorf("marshal init output: %w", err)
	}
	writeMessage(ctx, conn, initBytes)

	if err := server.Services.BroadcastGamesEvent(ctx, SerializeGameOutputPlayers(
		gameID,
		svc.SerializePlayer(s.WhitePlayer),
		svc.SerializePlayer(s.BlackPlayer),
	)); err != nil {
		return player, err
	}

	return player, nil
}

func (server *Server) handleGameMessage(ctx GameSocketContext, input message) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(input, &pbInput); err != nil {
		ctx.ErrChan <- err
		return
	}

	slog.InfoContext(ctx.Context, "received game input", "pbInput", &pbInput)

	var err error
	switch p := pbInput.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		err = server.handleGameForfeit(ctx)
	case *pb.GameInput_Move:
		err = server.handleGameMove(ctx, p.Move)
	case *pb.GameInput_Chat:
		err = server.handleGameChat(ctx, p.Chat)
	case *pb.GameInput_Undo:
		err = server.handleGameUndo(ctx, p.Undo)
	case *pb.GameInput_Ping:
		// no-op or heartbeat
	default:
		err = ErrWsMessageType
	}
	if err != nil {
		ctx.ErrChan <- err
	}
}

func (server *Server) handleGameForfeit(ctx GameSocketContext) error {
	endState, err := server.Services.EndGame(ctx.Context, ctx.GameID, ctx.Player)
	if err != nil {
		return fmt.Errorf("forfeit game %s: %w", ctx.GameID, err)
	}
	return server.BroadcastGamesEvent(ctx.Context, SerializeGameOutputForfeit(
		ctx.GameID,
		endState,
	))
}

func (server *Server) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput) error {
	moveResult, err := server.Services.MakeGameMove(ctx.Context, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return fmt.Errorf("make move on game %s: %w", ctx.GameID, err)
	}

	return server.Services.BroadcastGamesEvent(ctx.Context, SerializeGameOutputMove(
		ctx.GameID,
		chess.SerializeHistMove(moveResult.Move),
		chess.SerializeGame(&moveResult.State.Game), server.EntropySource.GetNow(),
	))
}

func (server *Server) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput) error {
	outputChat, chatMsg := SerializeGameOutputChat(
		ctx.GameID,
		pbInput.Message,
		ctx.Player,
		server.EntropySource.GetNow(),
	)
	if err := server.Services.InsertStateChat(ctx.Context, ctx.GameID, chatMsg); err != nil {
		return fmt.Errorf("insert chat on game %s: %w", ctx.GameID, err)
	}
	return server.Services.BroadcastGamesEvent(ctx.Context, outputChat)
}

func (server *Server) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput) error {
	undoKind, err := DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	state, err := server.Services.AttemptGameUndo(ctx.Context, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return fmt.Errorf("%+v attempting undo on game %s: %w", ctx.Player, ctx.GameID, err)
	}

	return server.Services.BroadcastGamesEvent(ctx.Context, SerializeGameOutputUndo(
		ctx.GameID,
		pbInput.Kind,
		ctx.Player.ID,
		state,
	))
}
