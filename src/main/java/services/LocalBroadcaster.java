package services;

import com.github.benmanes.caffeine.cache.Caffeine;
import com.github.benmanes.caffeine.cache.LoadingCache;
import com.github.benmanes.caffeine.cache.Scheduler;
import io.jooby.WebSocket;

import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;

import static utils.Globals.LOGGER;

public class LocalBroadcaster implements Broadcaster {

    private final LoadingCache<String, List<WebSocket>> socketsMap = Caffeine.newBuilder()
        .scheduler(Scheduler.systemScheduler())
        .build(key -> new CopyOnWriteArrayList<>());

    @Override
    public void subscribe(String id, WebSocket ws) {
        var socketList = socketsMap.get(id);
        socketList.add(ws);
        socketsMap.put(id, socketList);
        LOGGER.info("Ws {} subscribed to id: {}, ref: {}", ws.toString(), id, this);
    }

    @Override
    public void unsubscribe(String id, WebSocket ws) {
        var socketList = socketsMap.get(id);
        socketList.remove(ws);
        socketsMap.put(id, socketList);
        LOGGER.info("Ws {} unsubscribed from id: {}, ref: {}", ws.toString(), id, this);
    }

    @Override
    public void broadcast(String id, String content) {
        var socketList = socketsMap.get(id);
        if (socketList == null) {
            LOGGER.info("Broadcast local to id: {}, but there where no subscribers", id);
            return;
        }
        socketList.forEach((socket) -> socket.send(content));
        LOGGER.info("Broadcast local to id: {}, content: {}, ref: {}", id, content, this);
    }
}
