package web.controllers;

import chess.Move;
import io.jooby.jackson.JacksonModule;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import io.jooby.*;
import models.state.GameState;
import models.entities.PlayerEntity;
import services.game.GameService;
import web.State;
import web.dto.GameInputMsg;
import web.dto.GameOutputMsg;

import java.util.UUID;

import static utils.Globals.*;
import static web.WebConstants.*;

public class WsController extends Jooby {

    private final DictionaryDao dictionaryDao;
    private final Broadcaster gameBroadcaster;
    private final GameService gameService;

    public WsController(State state) {
        this.dictionaryDao = state.getDictionaryDao();
        this.gameBroadcaster = state.getGameBroadcaster();
        this.gameService = state.getGameService();

        install(new JacksonModule(JSON_MAPPER));

        ws("/connections/games/{id}", this::onJoin);
    }

    public void onJoin(Context ctx, WebSocketConfigurer configurer) {
        String sessionId = ctx.query("sessionId").valueOrNull(); // query is safe for secrets over a websocket when using wss
        String gameId = ctx.path("id").value("");
        String wsId = UUID.randomUUID().toString();

        LOGGER.info("Player joined ws with id={} with sessionId={} to gameId={}", wsId, sessionId, gameId);

        PlayerEntity player = dictionaryDao.getSessionOrDefault(sessionId);

        configurer.onConnect(handleGameConnect(gameId, player, wsId));

        configurer.onMessage(handleGameMessage(gameId, player));

        configurer.onClose((ws, statusCode) -> gameBroadcaster.unsubscribe(gameId, wsId));
    }

    public WebSocket.OnConnect handleGameConnect(String gameId, PlayerEntity player, String wsId) {
        return ws -> EXECUTOR.execute(() -> {
            try {
                if (player == null) {
                    ws.render(GameOutputMsg.ofError(ERROR_SESSION_EXPIRED));
                    ws.close();
                    return;
                }
                LOGGER.info("Player {} attempting to connect to game {}", player.id, gameId);

                GameState gameState = gameService.join(gameId, player);
                if (gameState == null) {
                    ws.render(GameOutputMsg.ofError(ERROR_INVALID_GAME));
                    ws.close();
                    return;
                }

                ws.render(GameOutputMsg.ofConnect(gameState));

                gameBroadcaster.subscribe(gameState.id, wsId, ws::send);

                String jsonResult = JSON_MAPPER.writeValueAsString(GameOutputMsg.ofJoin(player));
                gameBroadcaster.broadcast(gameState.id, jsonResult);

                LOGGER.info("Player {} successfully connected to game {}", player.id, gameId);
            } catch (Exception e) {
                LOGGER.error("Fatal exception occurred", e);
                ws.close();
            }
        });
    }

    public WebSocket.OnMessage handleGameMessage(String gameId, PlayerEntity player) {
        return (ws, message) -> EXECUTOR.execute(() -> {
           LOGGER.info("Received message from player {}, {} on game {}", player.id, message.value(), gameId);
            try {
                try {
                    GameInputMsg input = JSON_MAPPER.readValue(message.value(), GameInputMsg.class);
                    int type = input.getType();
                    switch (type) {
                    case GameInputMsg.FORFEIT -> {
                        GameState game = gameService.forfeit(gameId, player);
                        String jsonOutput = JSON_MAPPER.writeValueAsString(GameOutputMsg.ofForfeit(game));
                        gameBroadcaster.broadcast(gameId, jsonOutput);
                    }
                    case GameInputMsg.MOVE -> {
                        Move move = input.getMove();
                        GameState game = gameService.makeMove(gameId, player, move);

                        String jsonOutput = JSON_MAPPER.writeValueAsString(GameOutputMsg.ofMove(move, game));
                        gameBroadcaster.broadcast(gameId, jsonOutput);
                    }
                    case GameInputMsg.TEXT -> {
                        String jsonOutput = JSON_MAPPER.writeValueAsString(GameOutputMsg.ofText(input.getMessage()));
                        gameBroadcaster.broadcast(gameId, jsonOutput);
                    }
                    default -> ws.render(GameOutputMsg.ofError(ERROR_MESSAGE_TYPE));
                    }
                } catch (GameService.FinishedGameException e) {
                    ws.render(GameOutputMsg.ofError(ERROR_FINISHED_GAME));
                } catch (GameService.InvalidMoveException e) {
                    ws.render(GameOutputMsg.ofError(ERROR_INVALID_MOVE));
                } catch (GameService.TurnException e) {
                    ws.render(GameOutputMsg.ofError(ERROR_TURN));
                }
            } catch (Exception e) {
                LOGGER.error("Fatal exception occurred", e);
            }
        });
    }
}
