package web.controllers;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.jackson.JacksonModule;
import models.entities.PlayerEntity;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import web.reusable.AuthService;
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
//        Broadcaster userBroadcaster = state.getUserBroadcaster();
//        AuthService authService = state.getAuthService();
//        DictionaryDao dictionaryDao = state.getDictionaryDao();
//
//        Context ctx = sse.getContext();
//
//        // silently close the sse if we have auth issues, we cannot deliver notifications
//        AuthService.UserSession session = authService.parseSession(ctx);
//        if (session == null) {
//            sse.close();
//            return;
//        }
//        PlayerEntity player = dictionaryDao.getSession(session.sessionId);
//        if (player == null) {
//            sse.close();
//            return;
//        }
//
//        String sseId = UUID.randomUUID().toString();
//        String userId = Long.toString(player.id);
//
//        userBroadcaster.subscribe(userId, sseId, sse::send);
//        sse.onClose(() -> userBroadcaster.unsubscribe(userId, sseId));
    }
}
