package network

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/game"
	"hexchess-svc/gameid"
	"hexchess-svc/player"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"hexchess-svc/pb"
)

type GameSocketContext struct {
	context.Context
	GameID  gameid.Identifier
	Player  player.PlayerState
	ErrChan chan websocketError
}

type websocketError struct {
	// messageID allows a client to pair which input wsMessage led to which output error
	// it is left blank if the output cannot be traced or is a system-originated error
	messageID string
	value     error
}

// GameplayChanBufCap start dropping messages when a WebSocket is behind by this many messages
// must be higher than the number of messages broadcasted in initialization for tests to be deterministic
const GameplayChanBufCap = 10

func (server *HttpServer) HandleGameWebSocket(w http.ResponseWriter, r *http.Request) {
	// initialize static data
	ctx := r.Context()

	query := r.URL.Query()
	gameID := gameid.Identifier(query.Get("gameId"))
	sessionID := query.Get("sessionId")

	subChan := make(chan wsMessage, GameplayChanBufCap)
	errChan := make(chan websocketError)

	// initialize websocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "failed to upgrade ws connection", "error", err)
		return
	}

	cleanup := sync.OnceFunc(func() {
		server.broadcasters.Games.Unsubscribe(gameID, subChan) // send close signal to writer
		conn.Close()                                           // send close signal to reader
	})

	// initialize connection state (begin listening, get session data, sent initial responses etc.)
	server.broadcasters.Games.Subscribe(gameID, subChan)

	playerState, messages := server.handleGameInit(ctx, gameID, sessionID)
	for _, message := range messages {
		writeMessage(ctx, conn, message)
	}

	// start input reader (close signal received from client)
	go func() {
		ctx := GameSocketContext{Context: context.WithoutCancel(ctx), GameID: gameID, Player: playerState, ErrChan: errChan}
		defer cleanup()
		for {
			_, input, err := conn.ReadMessage()
			if err != nil {
				// received close signal from reader
				slog.InfoContext(ctx, "websocket read error", "gameID", gameID, "error", err)
				break
			}
			go server.handleGameMessage(ctx, input)
		}
	}()

	// start output writer (close signal received from server)
	go func() {
		defer cleanup()
		for {
			select {
			case wsErr := <-errChan:
				writeMessage(ctx, conn, marshalGameError(ctx, GameError{GameID: gameID, MessageID: wsErr.messageID, Error: wsErr.value}))
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

type GameError = game.ErrorGameOutput

func marshalGameError(ctx context.Context, output GameError) []byte {
	err := output.Error

	var wsErr error
	switch {
	case errutil.IsType[WsMessageTypeError](err):
		wsErr = ErrWsMessageType
	case errors.Is(err, game.ErrNoChessState):
		wsErr = ErrWsInvalidGame
	case errutil.IsType[game.ErrFinishedGame](err):
		wsErr = ErrWsFinishedGame
	case errors.Is(err, game.ErrForfeitPlayer):
		wsErr = ErrWsForfeitPlayer
	case errutil.IsType[game.ErrStartedGame](err):
		wsErr = ErrWsStartedGame
	case errutil.IsType[game.ErrTurn](err):
		wsErr = ErrWsTurn
	case errutil.IsType[game.ErrInvalidMove](err):
		wsErr = ErrWsInvalidMove
	case errors.Is(err, game.ErrNoChessState):
		// if the state cannot be found, it has expired while an inactive connection has been open
		wsErr = ErrWsExpiration
	case errors.Is(err, game.ErrUndoCurrPlayer):
		wsErr = ErrWsUndoCurrPlayer
	case
		errors.Is(err, game.ErrNoMoveUndo),
		errors.Is(err, game.ErrUndoNoop),
		errors.Is(err, game.ErrNoUndo):
		wsErr = ErrWsUndoAction
	default:
		wsErr = ErrWsFatal
	}

	level := slog.LevelWarn
	if errors.Is(wsErr, ErrWsFatal) {
		level = slog.LevelError
	}
	slog.Log(ctx, level, "failed to handle ws message", "error", err, "wsErr", wsErr, "messageID", output.MessageID)

	bytes, err := game.MarshalGameOutputError(game.ErrorGameOutput{GameID: output.GameID, MessageID: output.MessageID, Error: wsErr})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal err output", "error", err)
		bytes = nil
	}
	return bytes
}

func (server *HttpServer) handleGameInit(ctx context.Context, gameID gameid.Identifier, sessionID string) (player.PlayerState, []wsMessage) {
	errorResponse := func(err error) (player.PlayerState, []wsMessage) {
		messages := []wsMessage{
			marshalGameError(ctx, GameError{GameID: gameID, Error: err}),
		}
		return player.PlayerState{}, messages
	}

	playerState, err := server.service.GetSession(ctx, sessionID)
	if err != nil {
		return errorResponse(serrors.New("get session in game init phase", err))
	}
	chessState, err := server.service.JoinGame(ctx, gameID, playerState)
	if err != nil {
		return errorResponse(serrors.New("join game in init game phase", err))
	}

	initBytes, err := game.MarshalGameOutputInit(game.InitGameOutput{GameID: gameID, State: chessState, Self: playerState})
	if err != nil {
		return errorResponse(serrors.New("marshal init output", err))
	}

	output := game.SerializeGameOutputPlayers(game.PlayersGameOutput{
		GameID:      gameID,
		WhitePlayer: chessState.WhitePlayer,
		BlackPlayer: chessState.BlackPlayer,
	})
	server.broadcaster.BroadcastGamesEvent(ctx, output)

	messages := []wsMessage{initBytes}
	return playerState, messages
}

type WsMessageTypeError struct {
	pbInput *pb.GameInput
}

func (err WsMessageTypeError) Error() string {
	return fmt.Sprintf("message type %T is invalid: %+v ", err.pbInput, err.pbInput.GetValue())
}

func (server *HttpServer) handleGameMessage(ctx GameSocketContext, input wsMessage) {
	var pbInput pb.GameInput
	if err := pbInput.UnmarshalVT(input); err != nil {
		ctx.ErrChan <- websocketError{value: err}
		return
	}

	ctx.Context = slogutil.SetCtxMapKey(ctx.Context, slogutil.MessageID, pbInput.MessageId)

	slog.InfoContext(ctx, "received game input", "pbInputType", fmt.Sprintf("%T", &pbInput), "pbInput", &pbInput)

	var err error
	switch p := pbInput.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		err = server.handleGameForfeit(ctx, pbInput.MessageId)
	case *pb.GameInput_Move:
		err = server.handleGameMove(ctx, p.Move, pbInput.MessageId)
	case *pb.GameInput_Chat:
		err = server.handleGameChat(ctx, p.Chat, pbInput.MessageId)
	case *pb.GameInput_Undo:
		err = server.handleGameUndo(ctx, p.Undo, pbInput.MessageId)
	case *pb.GameInput_Ping:
		// no-op or heartbeat
	default:
		err = WsMessageTypeError{pbInput: &pbInput}
	}
	if err != nil {
		ctx.ErrChan <- websocketError{messageID: pbInput.MessageId, value: err}
	}
}

func (server *HttpServer) handleGameForfeit(ctx GameSocketContext, messageID string) error {
	endState, err := server.service.EndGame(ctx, ctx.GameID, ctx.Player)
	if err != nil {
		return serrors.New("forfeit game", err, "gameID", ctx.GameID)
	}

	output := game.SerializeGameOutputForfeit(game.ForfeitGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		EndState:  endState,
	})
	server.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}

func (server *HttpServer) handleGameMove(ctx GameSocketContext, pbInput *pb.MoveInput, messageID string) error {
	moveResult, err := server.service.MakeGameMove(ctx, ctx.GameID, ctx.Player, chess.DeserializeMove(pbInput.Move))
	if err != nil {
		return serrors.New("make move on game", err, "gameID", ctx.GameID)
	}

	output := game.SerializeGameOutputMove(game.MoveGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		Move:      moveResult.Move,
		State:     moveResult.State,
		UpdatedAt: time.Now(),
	})
	server.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}

func (server *HttpServer) handleGameChat(ctx GameSocketContext, pbInput *pb.ChatInput, messageID string) error {
	chat := game.Chat{
		Player:  ctx.Player,
		Message: pbInput.Message,
		SentAt:  time.Now(),
	}
	chat, err := server.service.InsertChat(ctx, ctx.GameID, chat)
	if err != nil {
		return serrors.New("insert chat on game", err, "gameID", ctx.GameID)
	}

	outputChat := game.SerializeGameOutputChat(game.ChatGameOutput{GameID: ctx.GameID, MessageID: messageID, Chat: chat})
	server.broadcaster.BroadcastGamesEvent(ctx, outputChat)
	return nil
}

func (server *HttpServer) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput, messageID string) error {
	undoKind, err := game.DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	chessState, err := server.service.AttemptGameUndo(ctx, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return serrors.New("attempting undo on game", err, "player", ctx.Player, "gameID", ctx.GameID)
	}

	output := game.SerializeGameOutputUndo(game.UndoGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		Kind:      pbInput.Kind,
		UndoID:    ctx.Player.ID,
		State:     chessState,
	})
	server.broadcaster.BroadcastGamesEvent(ctx, output)
	return nil
}
