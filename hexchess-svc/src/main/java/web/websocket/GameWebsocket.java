package web.websocket;

import chess.ChessGame;
import chess.PieceMove;
import io.jooby.WebSocket;
import io.jooby.WebSocketCloseStatus;
import io.jooby.WebSocketMessage;
import lombok.AllArgsConstructor;
import messages.Messages;
import models.state.Player;
import models.state.ChessRoom;
import services.broadcast.GroupBroadcaster;
import services.broadcast.BroadcastReceiver;
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
                ws.sendBinary(serializeError(ERROR_SESSION_EXPIRED));
                ws.close();
                return;
            }

            LOG.info("Player {} attempting to connect to game {}", player.getId(), gameId);

            ChessRoom room = gameService.join(gameId, player);
            if (room == null) {
                ws.sendBinary(serializeError(ERROR_INVALID_GAME));
                ws.close();
                return;
            }

            byte[] initOutput = serializeStart(player, room);
            byte[] playersOutput = serializePlayers(room.getWhitePlayer(), room.getBlackPlayer());

            ws.sendBinary(initOutput);

            gameBroadcaster.subscribe(room.getId(), new BroadcastReceiver<>(wsId) {
                @Override
                public void onMessage(byte[] content) {
                    ws.sendBinary(content);
                }

                @Override
                public void onEviction() {
                    if (ws.isOpen()) {
                        LOG.info("Closed websocket with id={} during eviction process", wsId);
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
            ws.sendBinary(serializeError(ERROR_UNKNOWN));
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

                byte[] output = serializeForfeit();
                gameBroadcaster.broadcast(gameId, output);
            }
            case "MOVE" -> {
                PieceMove move = input.move();
                Objects.requireNonNull(move);

                GameService.MakeMoveResult result = gameService.makeMove(gameId, player, move);

                byte[] output = serializeMove(result.move(), result.room().getGame());
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
        GroupBroadcaster gameBroadcaster = state.getGameBroadcaster();

        LOG.info("Closed websocket with id={} with closeStatus={}", wsId, statusCode);
        gameBroadcaster.unsubscribe(gameId, wsId);
    }

    public static byte[] serializeError(String message) {
        return Messages.GameOutput.newBuilder()
            .setError(Messages.Error.newBuilder().setMessage(message))
            .build()
            .toByteArray();
    }

    public static byte[] serializeStart(Player self, ChessRoom room) {
        Messages.Init.Builder init = Messages.Init.newBuilder()
            .setRoom(room.serialize());
        if (self != null) {
            init.setSelf(self.serialize());
        }
        return Messages.GameOutput.newBuilder()
            .setInit(init.build())
            .build()
            .toByteArray();
    }

    public static byte[] serializePlayers(Player whitePlayer, Player blackPlayer) {
        Messages.Players.Builder players = Messages.Players.newBuilder();
        if (whitePlayer != null) {
            players.setWhitePlayer(whitePlayer.serialize());
        }
        if (blackPlayer != null) {
            players.setBlackPlayer(blackPlayer.serialize());
        }
        return Messages.GameOutput.newBuilder()
            .setPlayers(players.build())
            .build()
            .toByteArray();
    }

    public static byte[] serializeMove(PieceMove pieceMove, ChessGame game) {
        Messages.Move move = Messages.Move.newBuilder()
            .setPieceMove(pieceMove.serialize())
            .setGame(game.serialize())
            .build();
        return Messages.GameOutput.newBuilder()
            .setMove(move)
            .build()
            .toByteArray();
    }

    public static byte[] serializeChat(Player player, String message) {
        Messages.Chat chat = Messages.Chat.newBuilder()
            .setPlayer(player.serialize())
            .setMessage(message)
            .build();
        return Messages.GameOutput.newBuilder()
            .setChat(chat)
            .build()
            .toByteArray();
    }

    public static byte[] serializeForfeit() {
        return Messages.GameOutput.newBuilder()
            .setForfeit(Messages.Forfeit.newBuilder())
            .build()
            .toByteArray();
    }
}