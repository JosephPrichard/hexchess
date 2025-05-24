package web.websocket;

import chess.Move;
import io.jooby.WebSocket;
import io.jooby.WebSocketCloseStatus;
import io.jooby.WebSocketMessage;
import lombok.AllArgsConstructor;
import models.entities.PlayerEntity;
import models.state.ChessRoom;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import services.game.GameService;
import web.State;

import java.util.concurrent.atomic.AtomicReference;

import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOG;
import static web.WebConstants.*;
import static web.WebConstants.ERROR_UNKNOWN;

@AllArgsConstructor
public class GameWebsocket {
    private final State state;
    private final String sessionId;
    private final String gameId;
    private final String wsId;
    private AtomicReference<PlayerEntity> selfPlayer = new AtomicReference<>();

    public GameWebsocket(State state, String wsId, String gameId, String sessionId) {
        this.state = state;
        this.wsId = wsId;
        this.gameId = gameId;
        this.sessionId = sessionId;
    }

    public void onConnect(WebSocket ws) {
        DictionaryDao dictionaryDao = state.getDictionaryDao();
        GameService gameService = state.getGameService();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        try {
            PlayerEntity player = dictionaryDao.getSessionOrDefault(sessionId);
            selfPlayer.set(player);

            if (player == null) {
                LOG.warn("Invalid session {} when attempting to connect to game {}", sessionId, gameId);
                ws.render(new GameOutput.Error(ERROR_SESSION_EXPIRED));
                ws.close();
                return;
            }

            LOG.info("Player {} attempting to connect to game {}", player.getId(), gameId);

            ChessRoom chessRoom = gameService.join(gameId, player);
            if (chessRoom == null) {
                ws.render(new GameOutput.Error(ERROR_INVALID_GAME));
                ws.close();
                return;
            }

            ws.render(new GameOutput.Start(player, chessRoom));

            gameBroadcaster.subscribe(chessRoom.getId(), wsId, ws::send);

            String jsonResult = JSON_MAPPER.writeValueAsString(new GameOutput.Join(chessRoom.getWhitePlayer(), chessRoom.getBlackPlayer(), player));
            gameBroadcaster.broadcast(chessRoom.getId(), jsonResult);

            LOG.info("Player {} successfully connected to game {}", player.getId(), gameId);
        } catch (Exception e) {
            LOG.error("Fatal exception occurred", e);
            ws.render(new GameOutput.Error(ERROR_UNKNOWN));
            ws.close();
        }
    }

    public void onMessage(WebSocket ws, WebSocketMessage message) {
        GameService gameService = state.getGameService();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        PlayerEntity player = selfPlayer.get();
        LOG.info("Received message from player {}, {} on game {}", player.getId(), message.value(), gameId);
        try {
            GameInput input = JSON_MAPPER.readValue(message.value(), GameInput.class);
            switch (input.getType()) {
            case "FORFEIT" -> {
                ChessRoom game = gameService.forfeit(gameId, player);
                String jsonOutput = JSON_MAPPER.writeValueAsString(new GameOutput.Forfeit(game));
                gameBroadcaster.broadcast(gameId, jsonOutput);
            }
            case "MOVE" -> {
                Move move = input.getMove();
                ChessRoom chessRoom = gameService.makeMove(gameId, player, move);

                String jsonOutput = JSON_MAPPER.writeValueAsString(new GameOutput.Move(move, chessRoom.getGame()));
                gameBroadcaster.broadcast(gameId, jsonOutput);
            }
            case "TEXT" -> {
                String content = input.getMessage();
                if (content.length() > 250) {
                    content = content.substring(0, 250);
                }
                String jsonOutput = JSON_MAPPER.writeValueAsString(new GameOutput.Chat(player, content));
                gameBroadcaster.broadcast(gameId, jsonOutput);
            }
            default -> ws.render(new GameOutput.Error(ERROR_MESSAGE_TYPE));
            }
        } catch (Exception e) {
            String error = switch (e) {
                case GameService.FinishedGameException ex -> ERROR_FINISHED_GAME;
                case GameService.InvalidMoveException ex -> ERROR_INVALID_MOVE;
                case GameService.TurnException ex -> ERROR_TURN;
                default -> {
                    LOG.warn("Error exception occurred in handling connection", e);
                    yield ERROR_UNKNOWN;
                }
            };
            ws.render(new GameOutput.Error(error));
        }
    }

    public void onClose(WebSocket ws, WebSocketCloseStatus statusCode) {
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        LOG.info("Closed websocket with id={} with closeStatus={}", wsId, statusCode);
        gameBroadcaster.unsubscribe(gameId, wsId);
    }
}