package services.broadcast;

import com.github.benmanes.caffeine.cache.Caffeine;
import com.github.benmanes.caffeine.cache.LoadingCache;
import com.github.benmanes.caffeine.cache.Scheduler;
import lombok.AllArgsConstructor;

import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.function.Consumer;

import static utils.Globals.LOGGER;

public class LocalBroadcaster implements Broadcaster {

    @AllArgsConstructor
    private static class Handler {
        String handlerId;
        Consumer<String> consumer;

        @Override
        public String toString() {
            return handlerId;
        }
    }

    private final String name;
    private final LoadingCache<String, List<Handler>> handlerMap = Caffeine.newBuilder()
        .scheduler(Scheduler.systemScheduler())
        .build(key -> new CopyOnWriteArrayList<>());

    public LocalBroadcaster(String name) {
        this.name = name;
    }

    @Override
    public void subscribe(String groupId, String handlerId, Consumer<String> consumer) {
        List<Handler> handlerList = handlerMap.get(groupId);
        handlerList.add(new Handler(handlerId, consumer));
        LOGGER.info("Subscribed to id={} on broadcaster {}", groupId, name);
    }

    @Override
    public void unsubscribe(String groupId, String handlerId) {
        List<Handler> handlerList = handlerMap.get(groupId);
        if (handlerList.removeIf((handler) -> handler.handlerId.equals(handlerId))) {
            LOGGER.info("Unsubscribed from id={} on broadcaster {}", groupId, name);
        }
    }

    @Override
    public void broadcast(String groupId, String content) {
        List<Handler> handlerList = handlerMap.get(groupId);
        if (handlerList == null) {
            LOGGER.info("Broadcast local to id={} on broadcaster {}, but there were no subscribers", groupId, name);
            return;
        }
        handlerList.forEach((handler) -> handler.consumer.accept(content));
        LOGGER.info("Broadcast local to id={}, handlerList={} on broadcaster {}", groupId, handlerList, name);
    }
}
