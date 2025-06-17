package web.websocket;

import chess.PieceMove;
import io.jooby.WebSocket;
import io.jooby.WebSocketCloseStatus;
import io.jooby.WebSocketMessage;
import lombok.AllArgsConstructor;
import models.state.Player;
import models.state.ChessRoom;
import services.broadcast.GroupBroadcaster;
import services.broadcast.Receiver;
import services.daos.DictionaryDao;
import services.game.GameService;
import web.State;

import java.util.Objects;
import java.util.concurrent.atomic.AtomicReference;

import static utils.Globals.*;
import static web.WebConstants.*;
import static web.WebConstants.ERROR_UNKNOWN;

@AllArgsConstructor
public class GameWebsocket {
    private final State state;
    private final String sessionId;
    private final String gameId;
    private final String wsId;
    private AtomicReference<Player> self = new AtomicReference<>();

    public GameWebsocket(State state, String wsId, String gameId, String sessionId) {
        this.state = state;
        this.wsId = wsId;
        this.gameId = gameId;
        this.sessionId = sessionId;
    }

    public record GameInput(String type, PieceMove move, String message) {}

    public void onConnect(WebSocket ws) {
        DictionaryDao dictionaryDao = state.getDictionaryDao();
        GameService gameService = state.getGameService();
        GroupBroadcaster gameBroadcaster = state.getGameBroadcaster();

        try {
            Player player = dictionaryDao.getSessionOrDefault(sessionId);
            self.set(player);

            if (player == null) {
                LOG.warn("Invalid session {} when attempting to connect to game {}", sessionId, gameId);
                ws.sendBinary(GameMessages.serializeError(ERROR_SESSION_EXPIRED));
                ws.close();
                return;
            }

            LOG.info("Player {} attempting to connect to game {}", player.getId(), gameId);

            ChessRoom room = gameService.join(gameId, player);
            if (room == null) {
                ws.sendBinary(GameMessages.serializeError(ERROR_INVALID_GAME));
                ws.close();
                return;
            }

            byte[] initOutput = GameMessages.serializeStart(player, room);
            byte[] playersOutput = GameMessages.serializePlayers(room.getWhitePlayer(), room.getBlackPlayer());

            ws.sendBinary(initOutput);

            gameBroadcaster.subscribe(room.getId(), new Receiver<>(wsId) {
                @Override
                public void onMessage(byte[] content) {
                    ws.sendBinary(content);
                }

                @Override
                public void onEviction() {
                    if (ws.isOpen()) {
                        ws.close();
                    }
                }

                @Override
                public boolean isClosed() {
                    return !ws.isOpen();
                }
            });
            gameBroadcaster.broadcast(room.getId(), playersOutput);

            LOG.info("Player {} successfully connected to game {}", player.getId(), gameId);
        } catch (Exception e) {
            LOG.error("Fatal exception occurred", e);
            ws.sendBinary(GameMessages.serializeError(ERROR_UNKNOWN));
            ws.close();
        }
    }

    public void onMessage(WebSocket ws, WebSocketMessage message) {
        GameService gameService = state.getGameService();
        GroupBroadcaster gameBroadcaster = state.getGameBroadcaster();

        Player player = self.get();
        LOG.info("Received message from player {}, {} on game {}", player.getId(), message.value(), gameId);
        try {
            GameInput input = JSON.readValue(message.value(), GameInput.class);
            switch (input.type()) {
            case "FORFEIT" -> {
                gameService.forfeit(gameId, player);

                byte[] output = GameMessages.serializeForfeit();
                gameBroadcaster.broadcast(gameId, output);
            }
            case "MOVE" -> {
                PieceMove move = input.move();
                Objects.requireNonNull(move);

                GameService.MakeMoveResult result = gameService.makeMove(gameId, player, move);

                byte[] output = GameMessages.serializeMove(result.move(), result.room().getGame());
                gameBroadcaster.broadcast(gameId, output);
            }
            case "TEXT" -> {
                String content = input.message();
                Objects.requireNonNull(content);

                if (content.length() > 250) {
                    content = content.substring(0, 250);
                }

                byte[] output = GameMessages.serializeChat(player, content);
                gameBroadcaster.broadcast(gameId, output);
            }
            default -> ws.sendBinary(GameMessages.serializeError(ERROR_MESSAGE_TYPE));
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
            ws.sendBinary(GameMessages.serializeError(error));
        }
    }

    public void onClose(WebSocket ws, WebSocketCloseStatus statusCode) {
        GroupBroadcaster gameBroadcaster = state.getGameBroadcaster();

        LOG.info("Closed websocket with id={} with closeStatus={}", wsId, statusCode);
        gameBroadcaster.unsubscribe(gameId, wsId);
    }
}