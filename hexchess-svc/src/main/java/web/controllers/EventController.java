package web.controllers;

import io.jooby.Context;
import io.jooby.Jooby;
import io.jooby.ServerSentEmitter;
import io.jooby.exception.StatusCodeException;
import io.jooby.jackson.JacksonModule;
import models.state.PlayerState;
import services.broadcast.BroadcastReceiver;
import services.broadcast.GroupBroadcaster;
import services.broadcast.SingleBroadcaster;
import services.daos.DictionaryDao;
import web.State;
import web.reusable.AuthService;

import java.util.UUID;
import java.util.concurrent.*;

import static utils.Globals.*;

public class EventController extends Jooby {
    private static final String USERS_COUNT_EVENT = "userCountEvents";
    private static final String GAMES_COUNT_EVENT = "gameCountEvents";
    private static final String USER_EVENT = "userEvents";
    private static final String META_EVENT = "meta";

    private final DictionaryDao dictionaryDao;
    private final AuthService authService;
    private final GroupBroadcaster userBroadcaster;
    private final SingleBroadcaster userCountBroadcaster;
    private final SingleBroadcaster gameCountBroadcaster;

    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    public static class SseReceiver<Content> extends BroadcastReceiver<Content> {
        private final ServerSentEmitter sse;
        private final String event;

        public SseReceiver(String id, ServerSentEmitter sse, String event) {
            super(id);
            this.sse = sse;
            this.event = event;
        }

        @Override
        public void onMessage(Content content) {
            sse.send(event, content);
        }

        @Override
        public void onEviction() {
            if (sse.isOpen()) {
                sse.close();
            }
        }

        @Override
        public boolean isClosed() {
            return !sse.isOpen();
        }
    }

    public EventController(State state) {
        dictionaryDao = state.getDictionaryDao();
        authService = state.getAuthService();
        userBroadcaster = state.getUserBroadcaster();
        userCountBroadcaster = state.getUserCountBroadcaster();
        gameCountBroadcaster = state.getGameCountBroadcaster();

        install(new JacksonModule(JSON));

        sse("/events/count", this::handleCount);
        sse("/events/user", this::handleUserEvents);
    }

    private void onCloseCount(String sseId, ScheduledFuture<?> task) {
        try {
            task.cancel(false);

            dictionaryDao.removeUser(sseId);
            long count = dictionaryDao.getUsersCount();

            userCountBroadcaster.unsubscribe(sseId);
            userCountBroadcaster.broadcast(Long.toString(count));
        } catch (Exception ex) {
            LOG.error("Failed to execute close count event handler", ex);
        }
    }

    public void handleCount(ServerSentEmitter sse) {
        String sseId = UUID.randomUUID().toString();

        // update and retrieve the count state
        dictionaryDao.addUser(sseId);
        long userCount = dictionaryDao.getUsersCount();
        long roomsCount = dictionaryDao.getRoomsCount();

        // subscribe to all updates on counts
        userCountBroadcaster.broadcast(Long.toString(userCount));
        userCountBroadcaster.subscribe(new SseReceiver<>(sseId, sse, USERS_COUNT_EVENT));
        gameCountBroadcaster.subscribe(new SseReceiver<>(sseId, sse, GAMES_COUNT_EVENT));

        ScheduledFuture<?> fut = scheduler.scheduleAtFixedRate(
            () -> EXECUTOR.execute(() -> dictionaryDao.addUser(sseId)),
            1,
            1,
            TimeUnit.MINUTES);

        sse.send(META_EVENT, "Connected");
        sse.send(USERS_COUNT_EVENT, userCount);
        sse.send(GAMES_COUNT_EVENT, roomsCount);

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> EXECUTOR.execute(() -> onCloseCount(sseId, fut)));
    }

    public void handleUserEvents(ServerSentEmitter sse) {
        Context ctx = sse.getContext();

        // silently close the sse if we have auth issues, we cannot deliver notifications
        PlayerState player;
        try {
            player = authService.getSessionPlayer(ctx);
        } catch (StatusCodeException ex) {
            sse.send(META_EVENT, ex.getMessage());
            sse.close();
            return;
        }

        String sseId = UUID.randomUUID().toString();
        String userId = Long.toString(player.getId());

        userBroadcaster.subscribe(userId, new SseReceiver<>(sseId, sse, USER_EVENT));

        sse.send(META_EVENT, "Connected");

        sse.keepAlive(15, TimeUnit.SECONDS);
        sse.onClose(() -> EXECUTOR.execute(() -> userBroadcaster.unsubscribe(userId, sseId)));
    }
}
