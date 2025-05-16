package web.controllers;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.jackson.JacksonModule;
import models.entities.PlayerEntity;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import services.producers.ChallengeProducer;
import web.dto.ChallengeMsg;
import web.reusable.AuthService;
import web.State;

import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static utils.Globals.JSON_MAPPER;
import static utils.Globals.LOGGER;
import static web.WebConstants.*;

public class EventController extends Jooby {
    private final State state;

    private final AtomicInteger numberOfConnections = new AtomicInteger(0);

    public EventController(State state) {
        this.state = state;

        install(new JacksonModule(JSON_MAPPER));

        post("/events/user/challenges/publish", this::publishChallenge);
        sse("/events/user/subscriptions", this::handleEvents);
    }

    private String publishChallenge(Context ctx) {
        ChallengeMsg body = ctx.body(ChallengeMsg.class);

        ChallengeProducer challengeProducer = state.getChallengeProducer();
        challengeProducer.broadcastChallenge(body);

        return "SUCCESS";
    }

    private void handleEvents(ServerSentEmitter sse) {
        Broadcaster userBroadcaster = state.getUserBroadcaster();
        AuthService authService = state.getAuthService();
        DictionaryDao dictionaryDao = state.getDictionaryDao();

        Context ctx = sse.getContext();

        // silently close the sse if we have auth issues, we cannot deliver notifications
        String sessionId = authService.parseSession(ctx);
        if (sessionId == null) {
            sse.send("meta", ERROR_REQUIRED_LOGIN);
            sse.close();
            return;
        }
        PlayerEntity player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            sse.send("meta", ERROR_SESSION_EXPIRED);
            sse.close();
            return;
        }

        String sseId = UUID.randomUUID().toString();
        String userId = Long.toString(player.id);

        int joinCount = numberOfConnections.incrementAndGet();
        LOGGER.info("Player={} connected to the user events as connection {}", player.id, joinCount);

        userBroadcaster.subscribe(userId, sseId, (m) -> sse.send("challenge", m));

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> {
            int leaveCount = numberOfConnections.decrementAndGet();
            LOGGER.info("Player={} disconnected from event, leaving {} connections", player.id, leaveCount);
            userBroadcaster.unsubscribe(userId, sseId);
        });
    }
}
