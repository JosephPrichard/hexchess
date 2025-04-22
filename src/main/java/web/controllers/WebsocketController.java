package web.controllers;

import chess.Move;
import io.jooby.jackson.JacksonModule;
import services.Broadcaster;
import services.RemoteDict;
import io.jooby.*;
import io.jooby.exception.StatusCodeException;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.GameState;
import models.PlayerEntity;
import services.GameService;
import web.State;

import java.util.UUID;

import static utils.Globals.*;

public class WebsocketController extends Jooby {

    private static final String ERROR_MESSAGE_TYPE = "ERROR_MESSAGE_TYPE";
    private static final String ERROR_TURN = "ERROR_TURN";
    private static final String ERROR_INVALID_MOVE = "ERROR_INVALID_MOVE";
    private static final String ERROR_FINISHED_GAME = "ERROR_FINISHED_GAME";
    private static final String ERROR_INVALID_GAME = "ERROR_INVALID_GAME";

    private final State state;

    public WebsocketController(State state) {
        this.state = state;

        install(new JacksonModule(JSON_MAPPER));

        ws("/games/{id}", this::onJoin);
    }

    public void onJoin(Context ctx, WebSocketConfigurer configurer) {
        RemoteDict remoteDict = state.getRemoteDict();
        Broadcaster gameBroadcaster = state.getGameBroadcaster();

        String sessionId = ctx.query("sessionId").valueOrNull(); // query is safe for secrets over a websocket when using wss
        Value gameIdSlug = ctx.path("id");
        if (gameIdSlug.isMissing()) {
            throw new StatusCodeException(StatusCode.BAD_REQUEST, "Invalid request: must contain id within slug");
        }

        String gameId = gameIdSlug.toString();
        PlayerEntity player = remoteDict.getSessionOrDefault(sessionId);

        if (player == null) {
            throw new RuntimeException("Expected player to be non null");
        }

        String wsId = UUID.randomUUID().toString();

        configurer.onConnect(handleGameConnect(gameId, player, wsId));

        configurer.onMessage(handleGameMessage(gameId, player));

        configurer.onClose((ws, statusCode) -> gameBroadcaster.unsubscribe(gameId, wsId));
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
        String code; // only used for error
        PlayerEntity player; // only used for join, says who the joining player is
        Move move; // only used for move
        GameState gameState; // the current state of the game being played

        static OutputMsg ofError(String message) {
            return new OutputMsg(ERROR, message, null, null, null);
        }

        static OutputMsg ofForfeit(GameState gameState) {
            return new OutputMsg(FORFEIT, null, null, null, gameState);
        }

        static OutputMsg ofJoin(PlayerEntity player, GameState gameState) {
            return new OutputMsg(JOIN, null, player, null, gameState);
        }

        static OutputMsg ofMove(GameState gameState, Move move) {
            return new OutputMsg(MOVE, null, null, move, gameState);
        }

        static OutputMsg ofText(String message) {
            return new OutputMsg(TEXT, message, null, null, null);
        }
    }

    public WebSocket.OnConnect handleGameConnect(String gameId, PlayerEntity player, String wsId) {
        return ws -> EXECUTOR.execute(() -> {
            GameService gameService = state.getGameService();
            Broadcaster broadcaster = state.getGameBroadcaster();

            try {
                LOGGER.info("Player {} attempting to connect to game {}", player.id, gameId);

                GameState gameState = gameService.join(gameId, player);
                if (gameState == null) {
                    ws.render(OutputMsg.ofError(ERROR_INVALID_GAME));
                    return;
                }
                broadcaster.subscribe(gameState.id, wsId, ws::send);

                String jsonResult = JSON_MAPPER.writeValueAsString(OutputMsg.ofJoin(player, gameState));
                broadcaster.broadcast(gameState.id, jsonResult);
                LOGGER.info("Player {} successfully connected to game {}", player.id, gameId);
            } catch (Exception e) {
                LOGGER.error("Fatal exception occurred: {}", e.getMessage());
                ws.close();
            }
        });
    }

    public WebSocket.OnMessage handleGameMessage(String gameId, PlayerEntity player) {
        return (ws, message) -> EXECUTOR.execute(() -> {
            GameService gameService = state.getGameService();
            Broadcaster broadcaster = state.getGameBroadcaster();

            LOGGER.info("Received message from player {}, {} on game {}", player.id, message.value(), gameId);
            try {
                try {
                    InputMsg input = JSON_MAPPER.readValue(message.value(), InputMsg.class);
                    int type = input.getType();
                    switch (type) {
                    case InputMsg.FORFEIT -> {
                        GameState game = gameService.forfeit(gameId, player);
                        String jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofForfeit(game));
                        broadcaster.broadcast(gameId, jsonOutput);
                    }
                    case InputMsg.MOVE -> {
                        Move move = input.getMove();
                        GameState game = gameService.makeMove(gameId, player, move);
                        String jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofMove(game, move));
                        broadcaster.broadcast(gameId, jsonOutput);
                    }
                    case InputMsg.TEXT -> {
                        String jsonOutput = JSON_MAPPER.writeValueAsString(OutputMsg.ofText(input.getMessage()));
                        broadcaster.broadcast(gameId, jsonOutput);
                    }
                    default -> ws.render(OutputMsg.ofError(ERROR_MESSAGE_TYPE));
                    }
                } catch (GameService.FinishedGameException e) {
                    ws.render(OutputMsg.ofError(ERROR_FINISHED_GAME));
                } catch (GameService.InvalidMoveException e) {
                    ws.render(OutputMsg.ofError(ERROR_INVALID_MOVE));
                } catch (GameService.TurnException e) {
                    ws.render(OutputMsg.ofError(ERROR_TURN));
                }
            } catch (Exception e) {
                LOGGER.error("Fatal exception occurred: {}", e.getMessage());
            }
        });
    }
}
