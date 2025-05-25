package web.controllers;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.jackson.JacksonModule;
import models.state.Player;
import services.broadcast.Broadcaster;
import services.daos.DictionaryDao;
import services.producers.ChallengeProducer;
import models.message.ChallengeMsg;
import web.reusable.AuthService;
import web.State;

import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static utils.Globals.JSON;
import static utils.Globals.LOG;
import static web.WebConstants.*;

public class EventController extends Jooby {
    private final State state;

    private final AtomicInteger numberOfConnections = new AtomicInteger(0);

    public EventController(State state) {
        this.state = state;

        install(new JacksonModule(JSON));

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
        Player player = dictionaryDao.getSession(sessionId);
        if (player == null) {
            sse.send("meta", ERROR_SESSION_EXPIRED);
            sse.close();
            return;
        }

        String sseId = UUID.randomUUID().toString();
        String userId = Long.toString(player.getId());

        int joinCount = numberOfConnections.incrementAndGet();
        LOG.info("Player={} connected to the user events as connection {}", player.getId(), joinCount);

        userBroadcaster.subscribe(userId, sseId, (m) -> sse.send("challenge", m));

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> {
            int leaveCount = numberOfConnections.decrementAndGet();
            LOG.info("Player={} disconnected from event, leaving {} connections", player.getId(), leaveCount);
            userBroadcaster.unsubscribe(userId, sseId);
        });
    }
}
