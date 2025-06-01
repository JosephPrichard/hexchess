package web.websocket;

import chess.ChessGame;
import chess.Hexagon;
import chess.Move;
import chess.PieceMove;
import com.fasterxml.jackson.core.JsonProcessingException;
import io.jooby.WebSocket;
import io.jooby.WebSocketCloseStatus;
import io.jooby.WebSocketMessage;
import lombok.AllArgsConstructor;
import messages.Messages;
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

    public void onConnect(WebSocket ws) {
        DictionaryDao dictionaryDao = state.getDictionaryDao();
        GameService gameService = state.getGameService();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        try {
            Player player = dictionaryDao.getSessionOrDefault(sessionId);
            selfPlayer.set(player);

            if (player == null) {
                LOG.warn("Invalid session {} when attempting to connect to game {}", sessionId, gameId);
                ws.sendBinary(serializeError(ERROR_SESSION_EXPIRED));
                ws.close();
                return;
            }

            LOG.info("Player {} attempting to connect to game {}", player.getId(), gameId);

            ChessRoom chessRoom = gameService.join(gameId, player);
            if (chessRoom == null) {
                ws.sendBinary(serializeError(ERROR_INVALID_GAME));
                ws.close();
                return;
            }

            ws.sendBinary(serializeStart(player, chessRoom));

            gameBroadcaster.subscribe(chessRoom.getId(), wsId, ws::sendBinary);

            byte[] output = serializeJoin(chessRoom.getWhitePlayer(), chessRoom.getBlackPlayer());
            gameBroadcaster.broadcast(chessRoom.getId(), output);

            LOG.info("Player {} successfully connected to game {}", player.getId(), gameId);
        } catch (Exception e) {
            LOG.error("Fatal exception occurred", e);
            ws.sendBinary(serializeError(ERROR_UNKNOWN));
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

                byte[] output = serializeForfeit();
                gameBroadcaster.broadcast(gameId, output);
            }
            case "MOVE" -> {
                Move move = input.move();
                Objects.requireNonNull(move);

                GameService.MakeMoveResult result = gameService.makeMove(gameId, player, move);

                byte[] output = serializeMove(result.pm(), result.room().getGame());
                gameBroadcaster.broadcast(gameId, output);
            }
            case "TEXT" -> {
                String content = input.message();
                Objects.requireNonNull(content);

                if (content.length() > 250) {
                    content = content.substring(0, 250);
                }

                byte[] output = serializeChat(player, content);
                gameBroadcaster.broadcast(gameId, output);
            }
            default -> ws.sendBinary(serializeError(ERROR_MESSAGE_TYPE));
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
            ws.sendBinary(serializeError(error));
        }
    }

    public void onClose(WebSocket ws, WebSocketCloseStatus statusCode) {
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        LOG.info("Closed websocket with id={} with closeStatus={}", wsId, statusCode);
        gameBroadcaster.unsubscribe(gameId, wsId);
    }

    private static byte[] serializeError(String message) {
        return Messages.GameOutput.newBuilder()
            .setError(Messages.Error.newBuilder().setMessage(message))
            .build()
            .toByteArray();
    }

    private static byte[] serializeStart(Player selfPlayer, ChessRoom chessRoom) {
        Messages.Start start = Messages.Start.newBuilder()
            .setSelfPlayer(selfPlayer.serialize())
            .setRoom(chessRoom.serialize())
            .build();
        return Messages.GameOutput.newBuilder()
            .setStart(start)
            .build()
            .toByteArray();
    }

    private static byte[] serializeJoin(Player whitePlayer, Player blackPlayer) {
        Messages.Join.Builder builder = Messages.Join.newBuilder();
        if (whitePlayer != null) {
            builder.setWhitePlayer(whitePlayer.serialize());
        }
        if (blackPlayer != null) {
            builder.setBlackPlayer(blackPlayer.serialize());
        }
        return Messages.GameOutput.newBuilder()
            .setJoin(builder.build())
            .build()
            .toByteArray();
    }

    private static byte[] serializeMove(PieceMove pieceMove, ChessGame game) {
        Messages.Move move = Messages.Move.newBuilder()
            .setPieceMove(pieceMove.serialize())
            .setGame(game.serialize())
            .build();
        return Messages.GameOutput.newBuilder()
            .setMove(move)
            .build()
            .toByteArray();
    }

    private static byte[] serializeChat(Player player, String message) {
        Messages.Chat chat = Messages.Chat.newBuilder()
            .setPlayer(player.serialize())
            .setMessage(message)
            .build();
        return Messages.GameOutput.newBuilder()
            .setChat(chat)
            .build()
            .toByteArray();
    }

    private static byte[] serializeForfeit() {
        return Messages.GameOutput.newBuilder()
            .setForfeit(Messages.Forfeit.newBuilder())
            .build()
            .toByteArray();
    }
}