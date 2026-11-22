package network

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/game"
	"hexchess-svc/gameid"
	"hexchess-svc/player"
	"hexchess-svc/pubsub/event"
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
	GameID      gameid.Identifier
	Player      player.PlayerState
	MessageChan chan wsMessage
}

func (server *HttpServer) HandleGameWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	gameID := gameid.Identifier(query.Get("gameId"))
	sessionID := query.Get("sessionId")

	writerDone := make(chan struct{}, 1)
	messageChan := make(chan wsMessage)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.WarnContext(ctx, "WS: failed to upgrade ws connection", "error", err)
		return
	}

	// initialize connection state (begin listening, get session data, sent initial responses etc.)
	subscriber, unsubscribe, err := server.pubsub.Subscribe(ctx, event.GamesChannel, gameID.String())
	if err != nil {
		slog.WarnContext(ctx, "WS: failed to connect to pubsub broker", "error", err)
		return
	}

	cleanup := sync.OnceFunc(func() {
		writerDone <- struct{}{} // send close signal to writer
		conn.Close()             // send close signal to reader
		unsubscribe()            // send close signal to pub/sub broker
	})

	playerState, messages := server.handleGameInit(ctx, gameID, sessionID)
	for _, message := range messages {
		writeMessage(ctx, conn, message)
	}

	// start input reader (close signal received from client)
	go func() {
		ctx := GameSocketContext{Context: context.WithoutCancel(ctx), GameID: gameID, Player: playerState, MessageChan: messageChan}
		defer cleanup()
		for {
			_, input, err := conn.ReadMessage()
			if err != nil {
				// received close signal from reader
				slog.InfoContext(ctx, "WS: websocket reader closed", "gameID", gameID, "error", err)
				break
			}
			go server.handleGameMessage(ctx, input)
		}
	}()

	// start output writer (close signal received from server or client)
	go func() {
		defer cleanup()
		for {
			select {
			case <-writerDone:
				slog.InfoContext(ctx, "WS: websocket writer closed from client", "gameID", gameID)
				return
			case message := <-messageChan:
				writeMessage(ctx, conn, message)
			case message, ok := <-subscriber:
				if !ok {
					slog.InfoContext(ctx, "WS: websocket writer closed from server", "gameID", gameID)
					return
				}
				writeMessage(ctx, conn, message)
			}
		}
	}()
}

func (server *HttpServer) handleGameInit(ctx context.Context, gameID gameid.Identifier, sessionID string) (player.PlayerState, []wsMessage) {
	errResp := func(err error) (player.PlayerState, []wsMessage) {
		return player.PlayerState{}, []wsMessage{marshalGameError(ctx, GameError{GameID: gameID, Error: err})}
	}

	playerState, err := server.service.GetSession(ctx, sessionID)
	if err != nil {
		return errResp(serrors.New("get session in game init phase", err))
	}
	chessState, err := server.service.JoinGame(ctx, gameID, playerState)
	if err != nil {
		return errResp(serrors.New("join game in init game phase", err))
	}

	initBytes, err := game.MarshalGameOutputInit(game.InitGameOutput{GameID: gameID, State: chessState, Self: playerState})
	if err != nil {
		return errResp(serrors.New("marshal init output", err))
	}

	output := game.SerializeGameOutputPlayers(game.PlayersGameOutput{
		GameID:      gameID,
		WhitePlayer: chessState.WhitePlayer,
		BlackPlayer: chessState.BlackPlayer,
	})
	game.PublishGamesEvent(ctx, server.pubsub, output)

	return playerState, []wsMessage{initBytes}
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
		ctx.MessageChan <- marshalGameError(ctx, GameError{GameID: ctx.GameID, Error: err})
		return
	}

	ctx.Context = slogutil.SetCtxMapKey(ctx.Context, slogutil.MessageID, pbInput.MessageId)

	slog.InfoContext(ctx, "WS: received game input", "pbInputType", fmt.Sprintf("%T", &pbInput), "pbInput", &pbInput)

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
		ctx.MessageChan <- marshalGameError(ctx, GameError{GameID: ctx.GameID, MessageID: pbInput.MessageId, Error: err})
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
	game.PublishGamesEvent(ctx, server.pubsub, output)
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
	game.PublishGamesEvent(ctx, server.pubsub, output)
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

	output := game.SerializeGameOutputChat(game.ChatGameOutput{GameID: ctx.GameID, MessageID: messageID, Chat: chat})
	game.PublishGamesEvent(ctx, server.pubsub, output)
	return nil
}

func (server *HttpServer) handleGameUndo(ctx GameSocketContext, pbInput *pb.UndoInput, messageID string) error {
	undoKind, err := game.DeserializeUndoInput(pbInput)
	if err != nil {
		return err
	}

	chessState, err := server.service.AttemptGameUndo(ctx, ctx.GameID, ctx.Player, undoKind)
	if err != nil {
		return serrors.New("WS: attempting undo on game", err, "player", ctx.Player, "gameID", ctx.GameID)
	}

	output := game.SerializeGameOutputUndo(game.UndoGameOutput{
		GameID:    ctx.GameID,
		MessageID: messageID,
		Kind:      pbInput.Kind,
		UndoID:    ctx.Player.ID,
		State:     chessState,
	})
	game.PublishGamesEvent(ctx, server.pubsub, output)
	return nil
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

	slog.WarnContext(ctx, "WS: failed to handle ws message", "error", err, "wsErr", wsErr, "messageID", output.MessageID)

	bytes, err := game.MarshalGameOutputError(game.ErrorGameOutput{GameID: output.GameID, MessageID: output.MessageID, Error: wsErr})
	if err != nil {
		slog.ErrorContext(ctx, "WS: failed to marshal err output", "error", err)
		bytes = nil
	}
	return bytes
}
