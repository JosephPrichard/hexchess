package controller

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/model"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

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
	// step 1: initialize static data
	ctx := r.Context()

	query := r.URL.Query()
	gameID := model.GameID(query.Get("gameId"))
	sessionID := query.Get("sessionId")

	subChan := make(chan message, GameplayChanBufCap)
	errChan := make(chan websocketError)

	// step 2: initialize websocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "error", err)
		return
	}

	close := func() {
		// note: both close operations are idempotent.
		defer api.broadcasters.Games.Unsubscribe(gameID, subChan) // send close signal to writer
		defer conn.Close()                                        // send close signal to reader
	}

	// step 3: initialize connection state (begin listening, get session data, etc.)
	api.broadcasters.Games.Subscribe(gameID, subChan)

	player, err := api.handleGameInit(ctx, gameID, sessionID, conn)
	if err != nil {
		writeGameError(ctx, conn, GameError{GameID: gameID, Error: err})
	}

	// step 4: start input reader (close signal received from client)
	go func() {
		readCtx := GameSocketContext{
			Context: context.WithoutCancel(ctx),
			GameID:  gameID,
			Player:  player,
			ErrChan: errChan,
		}
		defer close()
		for {
			_, input, err := conn.ReadMessage()
			if err != nil {
				// received close signal from reader
				slog.InfoContext(ctx, "websocket read error", "gameID", gameID, "error", err)
				break
			}
			go api.handleGameMessage(readCtx, input)
		}
	}()

	// step 5: start output writer (close signal received from server)
	go func() {
		defer close()
		for {
			select {
			case wsErr := <-errChan:
				writeGameError(ctx, conn, GameError{GameID: gameID, MessageID: wsErr.messageID, Error: wsErr.value})
			case message, ok := <-subChan:
				if !ok {
					// receive close signal from writer
					slog.InfoContext(ctx, "websocket writer closed", "gameID", gameID)
					return
				}
				writeMessage(ctx, conn, message)
			}
		}
	}()
}

type GameError = model.ErrorGameOutput

func writeGameError(ctx context.Context, conn *websocket.Conn, output GameError) {
	err := output.Error

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
	logutil.Error(ctx, level, "failed to handle ws message", err, "wsErr", wsErr, "messageID", output.MessageID)

	bytes, err := model.MarshalGameOutputError(model.ErrorGameOutput{GameID: output.GameID, MessageID: output.MessageID, Error: wsErr})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal err output", "error", err)
		bytes = nil
	}
	writeMessage(ctx, conn, bytes)
}

func (api *API) handleGameInit(ctx context.Context, gameID model.GameID, sessionID string, conn *websocket.Conn) (player model.PlayerState, err error) {
	player, err = api.services.GetSession(ctx, sessionID)
	if err != nil {
		return player, serrors.New("get session in game init phase", err)
	}
	chessState, err := api.services.JoinGame(ctx, gameID, player)
	if err != nil {
		return player, serrors.New("join game in init game phase", err)
	}

	initBytes, err := model.MarshalGameOutputInit(model.InitGameOutput{GameID: gameID, State: chessState, Self: player})
	if err != nil {
		return player, serrors.New("marshal init output", err)
	}
	writeMessage(ctx, conn, initBytes)

	output := model.SerializeGameOutputPlayers(model.PlayersGameOutput{
		GameID:      gameID,
		WhitePlayer: chessState.WhitePlayer,
		BlackPlayer: chessState.BlackPlayer,
	})
	api.broadcaster.BroadcastGamesEvent(ctx, output)

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
	if err := pbInput.UnmarshalVT(input); err != nil {
		ctx.ErrChan <- websocketError{value: err}
		return
	}

	ctx.Context =  context.WithValue(ctx.Context, logutil.MessageID, pbInput.MessageId)

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
		return serrors.New("forfeit game", err, "gameID", ctx.GameID)
	}

	output := model.SerializeGameOutputForfeit(model.ForfeitGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		EndState:  endState,
	})
	api.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}

func (api *API) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput, messageID string) error {
	moveResult, err := api.services.NewGameMove(ctx, ctx.GameID, ctx.Player, model.DeserializeMove(pbInput.Move))
	if err != nil {
		return serrors.New("make move on game", err, "gameID", ctx.GameID)
	}

	output := model.SerializeGameOutputMove(model.MoveGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		Move:      moveResult.Move,
		State:     moveResult.State,
		UpdatedAt: time.Now(),
	})
	api.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}

func (api *API) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput, messageID string) error {
	chat := model.Chat{
		ID:      uuid.NewString(),
		Player:  ctx.Player,
		Message: pbInput.Message,
		SentAt:  time.Now(),
	}
	outputChat := model.SerializeGameOutputChat(model.ChatGameOutput{GameID: ctx.GameID, MessageID: messageID, Chat: chat})

	if err := api.services.InsertChat(ctx, ctx.GameID, chat); err != nil {
		return serrors.New("insert chat on game", err, "gameID", ctx.GameID)
	}

	api.broadcaster.BroadcastGamesEvent(ctx, outputChat)
	return nil
}

func (api *API) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput, messageID string) error {
	undoKind, err := model.DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	chessState, err := api.services.AttemptGameUndo(ctx, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return serrors.New("attempting undo on game", err, "player", ctx.Player, "gameID", ctx.GameID)
	}

	output := model.SerializeGameOutputUndo(model.UndoGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		Kind:      pbInput.Kind,
		UndoID:    ctx.Player.ID,
		State:     chessState,
	})
	api.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}
