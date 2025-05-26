package web.websocket;

import chess.ChessGame;
import chess.Move;
import com.fasterxml.jackson.core.JsonProcessingException;
import io.jooby.WebSocket;
import io.jooby.WebSocketCloseStatus;
import io.jooby.WebSocketMessage;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.state.Player;
import models.state.ChessRoom;
import services.broadcast.Broadcaster;
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
    private AtomicReference<Player> selfPlayer = new AtomicReference<>();

    public GameWebsocket(State state, String wsId, String gameId, String sessionId) {
        this.state = state;
        this.wsId = wsId;
        this.gameId = gameId;
        this.sessionId = sessionId;
    }

    public record GameInput(String type, chess.Move move, String message) {}

    public record ForfeitOutput(String type, ChessRoom room) {
        public ForfeitOutput(ChessRoom room) {
            this("FORFEIT", room);
        }
    }

    public record StartOutput(String type, Player selfPlayer, ChessRoom room) {
        public StartOutput(Player selfPlayer, ChessRoom room) {
            this("START", selfPlayer, room);
        }
    }

    public record JoinOutput(String type, Player whitePlayer, Player blackPlayer) {
        public JoinOutput(Player whitePlayer, Player blackPlayer) {
            this("JOIN", whitePlayer, blackPlayer);
        }
    }

    public record MoveOutput(String type, chess.Move move, ChessGame game) {
        public MoveOutput(chess.Move move, ChessGame game) {
            this("MOVE", move, game);
        }
    }

    public record ChatOutput(String type, Player player, String message) {
        public ChatOutput(Player player, String message) {
            this("CHAT", player, message);
        }
    }

    public record ErrorOutput(String type, String message) {
        public ErrorOutput(String message) {
            this("ERROR", message);
        }
    }

    public void onConnect(WebSocket ws) {
        DictionaryDao dictionaryDao = state.getDictionaryDao();
        GameService gameService = state.getGameService();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        try {
            Player player = dictionaryDao.getSessionOrDefault(sessionId);
            selfPlayer.set(player);

            if (player == null) {
                LOG.warn("Invalid session {} when attempting to connect to game {}", sessionId, gameId);
                ws.sendBinary(serializeOutput(new ErrorOutput(ERROR_SESSION_EXPIRED)));
                ws.close();
                return;
            }

            LOG.info("Player {} attempting to connect to game {}", player.getId(), gameId);

            ChessRoom chessRoom = gameService.join(gameId, player);
            if (chessRoom == null) {
                ws.sendBinary(serializeOutput(new ErrorOutput(ERROR_INVALID_GAME)));
                ws.close();
                return;
            }

            ws.sendBinary(serializeOutput(new StartOutput(player, chessRoom)));

            gameBroadcaster.subscribe(chessRoom.getId(), wsId, ws::sendBinary);

            byte[] output = serializeOutput(new JoinOutput(chessRoom.getWhitePlayer(), chessRoom.getBlackPlayer()));
            gameBroadcaster.broadcast(chessRoom.getId(), output);

            LOG.info("Player {} successfully connected to game {}", player.getId(), gameId);
        } catch (Exception e) {
            LOG.error("Fatal exception occurred", e);
            ws.sendBinary(serializeOutput(new ErrorOutput(ERROR_UNKNOWN)));
            ws.close();
        }
    }

    public void onMessage(WebSocket ws, WebSocketMessage message) {
        GameService gameService = state.getGameService();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        Player player = selfPlayer.get();
        LOG.info("Received message from player {}, {} on game {}", player.getId(), message.value(), gameId);
        try {
            GameInput input = JSON.readValue(message.value(), GameInput.class);
            switch (input.type()) {
            case "FORFEIT" -> {
                ChessRoom room = gameService.forfeit(gameId, player);

                byte[] output = serializeOutput(new ForfeitOutput(room));
                gameBroadcaster.broadcast(gameId, output);
            }
            case "MOVE" -> {
                Move move = input.move();
                Objects.requireNonNull(move);

                ChessRoom room = gameService.makeMove(gameId, player, move);

                byte[] output = serializeOutput(new MoveOutput(move, room.getGame()));
                gameBroadcaster.broadcast(gameId, output);
            }
            case "TEXT" -> {
                String content = input.message();
                Objects.requireNonNull(content);

                if (content.length() > 250) {
                    content = content.substring(0, 250);
                }

                byte[] output = serializeOutput(new ChatOutput(player, content));
                gameBroadcaster.broadcast(gameId, output);
            }
            default -> ws.sendBinary(serializeOutput(new ErrorOutput(ERROR_MESSAGE_TYPE)));
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
            ws.sendBinary(serializeOutput(new ErrorOutput(error)));
        }
    }

    public void onClose(WebSocket ws, WebSocketCloseStatus statusCode) {
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        LOG.info("Closed websocket with id={} with closeStatus={}", wsId, statusCode);
        gameBroadcaster.unsubscribe(gameId, wsId);
    }

    private static byte[] serializeOutput(Object output) {
        try {
            return MESSAGE_PACK.writeValueAsBytes(output);
        } catch (JsonProcessingException ex) {
            LOG.error("Failed to serialize output message", ex);
            throw new RuntimeException(ex);
        }
    }
}