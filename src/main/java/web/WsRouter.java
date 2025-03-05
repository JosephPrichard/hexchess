package web;

import domain.Move;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.GameState;
import models.Player;
import services.GameService;

import static utils.Globals.*;

public class WsRouter extends Jooby {

    private final State state;

    public WsRouter(State state) {
        this.state = state;

        ws("/games/join/{id}", this::onJoin);
    }

    public void onJoin(Context ctx, WebSocketConfigurer configurer) {
        var remoteDict = state.getRemoteDict();
        var broadcastService = state.getBroadcaster();

        var sessionId = ctx.query("sessionId").valueOrNull(); // query is safe for secrets over a websocket when using wss
        var gameIdSlug = ctx.path("id");
        if (gameIdSlug.isMissing()) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Invalid request: must contain id within slug");
        }

        var gameId = gameIdSlug.toString();
        var player = remoteDict.getSessionOrDefault(sessionId);

        if (player == null) {
            throw new RuntimeException("Expected player to be non null");
        }

        configurer.onConnect(handleGameConnect(gameId, player));

        configurer.onMessage(handleGameMessage(gameId, player));

        configurer.onClose((ws, statusCode) -> broadcastService.unsubscribe(gameId, ws));
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    static class InputMsg {
        static final int FORFEIT = 0;
        static final int MOVE = 1;
        static final int TEXT = 2;

        int type;
        Move move;
        String message;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    static class OutputMsg {
        static final int ERROR = 0;
        static final int FORFEIT = 1;
        static final int JOIN = 2;
        static final int MOVE = 3;
        static final int TEXT = 5;

        int type;
        String message; // only used for error
        Player player; // only used for join, says who the joining player is
        Move move; // only used for move
        GameState gameState; // the current state of the game being played

        static OutputMsg ofError(String message) {
            return new OutputMsg(ERROR, message, null, null, null);
        }

        static OutputMsg ofForfeit(GameState gameState) {
            return new OutputMsg(FORFEIT, null, null, null, gameState);
        }

        static OutputMsg ofJoin(Player player, GameState gameState) {
            return new OutputMsg(JOIN, null, player, null, gameState);
        }

        static OutputMsg ofMove(GameState gameState, Move move) {
            return new OutputMsg(MOVE, null, null, move, gameState);
        }

        static OutputMsg ofText(String message) {
            return new OutputMsg(TEXT, message, null, null, null);
        }
    }

    public WebSocket.OnConnect handleGameConnect(String gameId, Player player) {
        return ws -> EXECUTOR.execute(() -> {
            var gameService = state.getGameService();
            var broadcaster = state.getBroadcaster();

            try {
                var gameState = gameService.join(gameId, player);
                if (gameState == null) {
                    // we cannot join. so just send an error and then disconnect
                    var jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofError("Invalid message type"));
                    ws.send(jsonOutput);
                    ws.close();
                    return;
                }
                broadcaster.subscribe(gameState.getId(), ws);
                // the joiner needs a snapshot of what the game actually looks like when joining!
                var jsonResult = JSON_MAPPER.writeValueAsString(OutputMsg.ofJoin(player, gameState));
//                ws.send(jsonResult);
                broadcaster.broadcast(gameState.getId(), jsonResult);
                LOGGER.info("Player {} connected to game {}", player.getId(), gameId);
            } catch (Exception e) {
                // if we encounter some unknown error or maybe json failure, we can't really do anything so just log and close the connection
                LOGGER.error("Fatal exception occurred: {}", e.getMessage());
                ws.close();
            }
        });
    }

    public WebSocket.OnMessage handleGameMessage(String gameId, Player player) {
        return (ws, message) -> EXECUTOR.execute(() -> {
            var gameService = state.getGameService();
            var broadcaster = state.getBroadcaster();

            LOGGER.info("Received message from player {}, {} on game {}", player.getId(), message.value(), gameId);
            try {
                try {
                    var input = JSON_MAPPER.readValue(message.value(), InputMsg.class);
                    var type = input.getType();
                    switch (type) {
                        case InputMsg.FORFEIT -> {
                            var game = gameService.forfeit(gameId, player);
                            var jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofForfeit(game));
                            broadcaster.broadcast(gameId, jsonOutput);
                        }
                        case InputMsg.MOVE -> {
                            var move = input.getMove();
                            var game = gameService.makeMove(gameId, player, move);
                            var jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofMove(game, move));
                            broadcaster.broadcast(gameId, jsonOutput);
                        }
                        case InputMsg.TEXT -> {
                            var jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofText(input.getMessage()));
                            broadcaster.broadcast(gameId, jsonOutput);
                        }
                        default -> {
                            var resp = OutputMsg.ofError("Invalid message type: %d" + type);
                            var jsonOutput = JSON_MAPPER.writeValueAsString(resp);
                            ws.send(jsonOutput);
                        }
                    }
                } catch (GameService.MoveException e) {
                    var resp = OutputMsg.ofError(e.getMessage());
                    var jsonOutput = JSON_MAPPER.writeValueAsString(resp);
                    ws.send(jsonOutput);
                }
            } catch (Exception e) {
                LOGGER.error("Fatal exception occurred: {}", e.getMessage());
            }
        });
    }
}
