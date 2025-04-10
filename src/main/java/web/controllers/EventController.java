package web.controllers;

import chess.Move;
import com.fasterxml.jackson.core.JsonProcessingException;
import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.jackson.JacksonModule;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import models.GameState;
import models.PlayerEntity;
import services.Broadcaster;
import services.RemoteDict;
import services.SessionService;
import web.State;

import java.util.UUID;

import static utils.Globals.JSON_MAPPER;

public class EventController extends Jooby {
    private final State state;

    public EventController(State state) {
        this.state = state;

        install(new JacksonModule(JSON_MAPPER));

        sse("/events/user", this::handleEvents);
    }

    private void handleEvents(ServerSentEmitter sse) {
        Broadcaster userBroadcaster = state.getUserBroadcaster();
        SessionService sessionService = state.getSessionService();
        RemoteDict remoteDict = state.getRemoteDict();

        Context ctx = sse.getContext();

        // silently close the sse if we have auth issues, we cannot deliver notifications
        SessionService.SessionValue session = sessionService.parseSession(ctx);
        if (session == null) {
            sse.close();
            return;
        }
        PlayerEntity player = remoteDict.getSession(session.getSessionId());
        if (player == null) {
            sse.close();
            return;
        }

        String sseId = UUID.randomUUID().toString();
        String userId = Long.toString(player.getId());

        userBroadcaster.subscribe(userId, sseId, sse::send);
        sse.onClose(() -> userBroadcaster.unsubscribe(userId, sseId));
    }
}
