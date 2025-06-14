package web.controllers;

import io.jooby.jackson.JacksonModule;
import io.jooby.*;
import web.State;
import web.websocket.GameWebsocket;

import java.util.UUID;
import java.util.concurrent.CompletableFuture;

import static utils.Globals.*;

public class WsController extends Jooby {

    private final State state;

    public WsController(State state) {
        this.state = state;

        install(new JacksonModule(JSON));

        ws("/connections/games/{id}", this::onJoin);
    }

    public void onJoin(Context ctx, WebSocketConfigurer configurer) {
        String sessionId = ctx.query("sessionId").valueOrNull(); // query is safe for secrets over a websocket when using wss
        String gameId = ctx.path("id").value("");
        String wsId = UUID.randomUUID().toString();

        LOG.info("Player joined ws with id={} with sessionId={} to gameId={}", wsId, sessionId, gameId);

        GameWebsocket websocket = new GameWebsocket(state, wsId, gameId, sessionId);

        configurer.onConnect((ws) -> CompletableFuture.runAsync(() -> websocket.onConnect(ws), EXECUTOR));

        configurer.onMessage((ws, message) -> CompletableFuture.runAsync(() -> websocket.onMessage(ws, message), EXECUTOR));

        configurer.onClose(websocket::onClose);
    }
}
