package controller

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/lib/logutil"

	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/pb"
	svc "hexchess-svc/service"

	"github.com/gorilla/websocket"
)

type GameSocketContext struct {
	context.Context
	GameID  model.GameID
	Player  model.PlayerState
	ErrChan chan websocketError
}

func (ctx GameSocketContext) WithMessageID(messageID string) GameSocketContext {
	return GameSocketContext{
		Context: context.WithValue(ctx, logutil.MessageID, messageID),
		GameID:  ctx.GameID,
		Player:  ctx.Player,
		ErrChan: ctx.ErrChan,
	}
}

type websocketError struct {
	// messageID allows a client to pair which input message led to which output error
	// it is left blank if the output cannot be traced or is a system-originated error
	messageID string
	value     error
}

// GameplayChanBufCap start dropping messages when a websocket is behind by this many messages
// must be higher than the number of messages broadcasted in initialization for tests to be deterministic
const GameplayChanBufCap = 10

func (api *API) HandleGameWs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := model.GameID(query.Get("gameId"))
	sessionID := query.Get("sessionId")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "error", err)
		return
	}
	defer conn.Close() // close originating from server

	subscriber := make(chan message, GameplayChanBufCap)

	api.broadcasters.GamesCaster.Subscribe(gameID, subscriber)
	defer api.broadcasters.GamesCaster.Unsubscribe(gameID, subscriber)

	player, err := api.handleGameInit(ctx, gameID, sessionID, conn)
	if err != nil {
		writeGameError(ctx, conn, gameID, "", err)
		return
	}

	errChan := make(chan websocketError)

	go func() {
		defer conn.Close() // close originating from client
		for {
			_, input, err := conn.ReadMessage()
			if err != nil {
				slog.WarnContext(ctx, "websocket read error", "gameID", gameID, "error", err)
				break
			}
			gameSocketCtx := GameSocketContext{Context: ctx, GameID: gameID, Player: player, ErrChan: errChan}
			go api.handleGameMessage(gameSocketCtx, input)
		}
	}()

	for {
		select {
		case wsErr := <-errChan:
			writeGameError(ctx, conn, gameID, wsErr.messageID, wsErr.value)
		case message, ok := <-subscriber:
			if !ok {
				slog.InfoContext(ctx, "websocket writer closed", "gameID", gameID)
				return
			}
			writeMessage(ctx, conn, message)
		}
	}
}

func writeGameError(ctx context.Context, conn *websocket.Conn, gameID model.GameID, messageID string, err error) {
	var wsErr error
	switch {
	case errutil.IsType[WsMessageTypeError](err):
		wsErr = ErrWsMessageType
	case errors.Is(err, svc.ErrNoChessState):
		wsErr = ErrWsInvalidGame
	case errutil.IsType[svc.ErrFinishedGame](err):
		wsErr = ErrWsFinishedGame
	case errors.Is(err, svc.ErrForfeitPlayer):
		wsErr = ErrWsForfeitPlayer
	case errutil.IsType[svc.ErrStartedGame](err):
		wsErr = ErrWsStartedGame
	case errutil.IsType[svc.ErrTurn](err):
		wsErr = ErrWsTurn
	case errutil.IsType[svc.ErrInvalidMove](err):
		wsErr = ErrWsInvalidMove
	case errors.Is(err, svc.ErrNoChessState):
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, svc.ErrUndoCurrPlayer):
		wsErr = ErrWsUndoCurrPlayer
	case
		errors.Is(err, model.ErrNoMoveUndo),
		errors.Is(err, svc.ErrUndoNoop),
		errors.Is(err, svc.ErrNoUndo):
		wsErr = ErrWsUndoAction
	default:
		wsErr = ErrWsFatal
	}

	level := slog.LevelWarn
	if wsErr == ErrWsFatal {
		level = slog.LevelError
	}
	logutil.SError(ctx, level, "failed to handle ws message", err, "wsErr", wsErr, "messageID", messageID)

	bytes, err := proto.Marshal(SerializeGameOutputError(gameID, messageID, wsErr))
	if err != nil {
		// log with a noop response
		slog.ErrorContext(ctx, "failed to marshal err output", "error", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func (api *API) handleGameInit(ctx context.Context, gameID model.GameID, sessionID string, conn *websocket.Conn) (player model.PlayerState, err error) {
	player, err = api.services.GetSession(ctx, sessionID)
	if err != nil {
		return player, serrors.Wrap("get session in game init phase", err)
	}
	chessState, err := api.services.JoinGame(ctx, gameID, player)
	if err != nil {
		return player, serrors.Wrap("join game in init game phase", err)
	}

	initBytes, err := proto.Marshal(SerializeGameOutputInit(
		gameID,
		model.SerializeChessState(chessState),
		model.SerializePlayer(player),
	))
	if err != nil {
		return player, serrors.Wrap("marshal init output", err)
	}
	writeMessage(ctx, conn, initBytes)

	api.broadcaster.BroadcastGamesEvent(ctx, SerializeGameOutputPlayers(
		gameID,
		model.SerializePlayer(chessState.WhitePlayer),
		model.SerializePlayer(chessState.BlackPlayer),
	))

	return player, nil
}

type WsMessageTypeError struct {
	pbInput *pb.GameInput
}

func (err WsMessageTypeError) Error() string {
	return fmt.Sprintf("message type %T is invalid: %+v ", err.pbInput, err.pbInput.GetValue())
}

func (api *API) handleGameMessage(ctx GameSocketContext, input message) {
	var pbInput pb.GameInput
	if err := proto.Unmarshal(input, &pbInput); err != nil {
		ctx.ErrChan <- websocketError{value: err}
		return
	}

	ctx = ctx.WithMessageID(pbInput.MessageId)
	slog.InfoContext(ctx, "received game input", "pbInputType", fmt.Sprintf("%T", &pbInput), "pbInput", &pbInput)

	var err error
	switch p := pbInput.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		err = api.handleGameForfeit(ctx, pbInput.MessageId)
	case *pb.GameInput_Move:
		err = api.handleGameMove(ctx, p.Move, pbInput.MessageId)
	case *pb.GameInput_Chat:
		err = api.handleGameChat(ctx, p.Chat, pbInput.MessageId)
	case *pb.GameInput_Undo:
		err = api.handleGameUndo(ctx, p.Undo, pbInput.MessageId)
	case *pb.GameInput_Ping:
		// no-op or heartbeat
	default:
		err = WsMessageTypeError{pbInput: &pbInput}
	}
	if err != nil {
		ctx.ErrChan <- websocketError{messageID: pbInput.MessageId, value: err}
	}
}

func (api *API) handleGameForfeit(ctx GameSocketContext, messageID string) error {
	endState, err := api.services.EndGame(ctx, ctx.GameID, ctx.Player)
	if err != nil {
		return serrors.Wrap("forfeit game", err, "gameID", ctx.GameID)
	}
	api.broadcaster.BroadcastGamesEvent(ctx, SerializeGameOutputForfeit(ctx.GameID, messageID, endState))
	return nil
}

func (api *API) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput, messageID string) error {
	moveResult, err := api.services.NewGameMove(ctx, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return serrors.Wrap("make move on game", err, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx, SerializeGameOutputMove(
		ctx.GameID,
		messageID,
		chess.SerializeHistMove(moveResult.Move),
		chess.SerializeGame(&moveResult.State.Game),
		time.Now(),
	))
	return nil
}

func (api *API) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput, messageID string) error {
	chatMsg := model.Chat{
		ID:      uuid.NewString(),
		Player:  ctx.Player,
		Message: pbInput.Message,
		SentAt:  time.Now(),
	}
	outputChat := SerializeGameOutputChat(ctx.GameID, messageID, chatMsg)

	if err := api.services.InsertChat(ctx, ctx.GameID, chatMsg); err != nil {
		return serrors.Wrap("insert chat on game", err, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx, outputChat)
	return nil
}

func (api *API) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput, messageID string) error {
	undoKind, err := DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	chessState, err := api.services.AttemptGameUndo(ctx, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return serrors.Wrap("attempting undo on game", err, "player", ctx.Player, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx, SerializeGameOutputUndo(ctx.GameID, messageID, pbInput.Kind, ctx.Player.ID, chessState))
	return nil
}
