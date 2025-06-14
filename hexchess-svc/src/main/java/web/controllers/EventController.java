package web.controllers;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.exception.StatusCodeException;
import io.jooby.jackson.JacksonModule;
import models.state.Player;
import services.broadcast.Broadcaster;
import services.broadcast.SingleBroadcaster;
import services.daos.DictionaryDao;
import web.State;
import web.reusable.AuthService;

import java.util.UUID;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static utils.Globals.*;

public class EventController extends Jooby {
    private static final String USERS_COUNT_EVENT = "userCountEvents";
    private static final String USER_EVENT = "userEvents";

    private final State state;

    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    private final AtomicInteger numberOfCountEvents = new AtomicInteger(0);
    private final AtomicInteger numberOfUserEvents = new AtomicInteger(0);

    public EventController(State state) {
        this.state = state;

        install(new JacksonModule(JSON));

        sse("/events/count", this::handleCount);
        sse("/events/user", this::handleUserEvents);
    }

    private void onCloseCount(Long userId, ScheduledFuture<?> fut) {
        SingleBroadcaster userCountBroadcaster = state.getUserCountBroadcaster();
        DictionaryDao dictionaryDao = state.getDictionaryDao();

        int leaveCount = numberOfCountEvents.decrementAndGet();
        LOG.info("Disconnected to counts as connection {}", leaveCount);

        fut.cancel(false);

        Long count = dictionaryDao.removeThenCountUsers(userId);
        if (count != null) {
            LOG.info("Decremented users count to {}", count);
            userCountBroadcaster.broadcast(Long.toString(count));
        }
    }

    private void handleCount(ServerSentEmitter sse) {
        SingleBroadcaster userCountBroadcaster = state.getUserCountBroadcaster();
        DictionaryDao dictionaryDao = state.getDictionaryDao();
        AuthService authService = state.getAuthService();

        Context ctx = sse.getContext();

        String sseId = UUID.randomUUID().toString();

        Player player = authService.getOptionalSessionPlayer(ctx);
        Long userId = player == null ? null : player.getId();

        int joinCount = numberOfCountEvents.incrementAndGet();
        LOG.info("Connected to counts as connection {}", joinCount);

        long count = dictionaryDao.addThenCountUsers(userId);
        LOG.info("Incremented users count to {}", count);

        sse.send(USERS_COUNT_EVENT, count);

        userCountBroadcaster.subscribe(sseId, (m) -> sse.send(USERS_COUNT_EVENT, m));

        // periodically refresh user as long as this sse is open
        ScheduledFuture<?> fut = scheduler.schedule(
            () -> CompletableFuture.runAsync(() -> dictionaryDao.addThenCountUsers(userId), EXECUTOR),
            1,
            TimeUnit.MINUTES);

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> CompletableFuture.runAsync(() -> onCloseCount(userId, fut), EXECUTOR));
    }

    private void onCloseUserEvent(Player player, String userId, String sseId) {
        Broadcaster userBroadcaster = state.getUserBroadcaster();

        int leaveCount = numberOfUserEvents.decrementAndGet();
        LOG.info("Player={} disconnected from event, leaving {} connections", player, leaveCount);

        userBroadcaster.unsubscribe(userId, sseId);
    }

    private void handleUserEvents(ServerSentEmitter sse) {
        Broadcaster userBroadcaster = state.getUserBroadcaster();
        AuthService authService = state.getAuthService();

        Context ctx = sse.getContext();

        // silently close the sse if we have auth issues, we cannot deliver notifications
        Player player;
        try {
            player = authService.getSessionPlayer(ctx);
        } catch (StatusCodeException ex) {
            sse.send("meta", ex.getMessage());
            sse.close();
            return;
        }

        String sseId = UUID.randomUUID().toString();
        String userId = Long.toString(player.getId());

        int joinCount = numberOfUserEvents.incrementAndGet();
        LOG.info("Player={} connected to the user events as connection {}", player.getId(), joinCount);

        userBroadcaster.subscribe(userId, sseId, (m) -> sse.send(USER_EVENT, m));

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> CompletableFuture.runAsync(() -> onCloseUserEvent(player, userId, sseId), EXECUTOR));
    }
}
