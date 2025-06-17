package services.broadcast;

import com.github.benmanes.caffeine.cache.Caffeine;
import com.github.benmanes.caffeine.cache.LoadingCache;
import com.github.benmanes.caffeine.cache.RemovalCause;
import com.github.benmanes.caffeine.cache.Scheduler;
import org.checkerframework.checker.nullness.qual.Nullable;

import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;

import static utils.Globals.LOG;

public class LocalBroadcaster implements Broadcaster {
    private final String name;
    private final LoadingCache<String, List<Receiver<byte[]>>> handlerMap = Caffeine.newBuilder()
        .scheduler(Scheduler.systemScheduler())
        .evictionListener(this::handleEviction)
        .expireAfterAccess(1, TimeUnit.HOURS)
        .build(key -> new CopyOnWriteArrayList<>());

    public LocalBroadcaster(String name) {
        this.name = name;
    }

    private void handleEviction(@Nullable String groupId, @Nullable List<Receiver<byte[]>> receiverList, RemovalCause cause) {
        if (receiverList != null) {
            LOG.info("Evicted groupId={} with receiverList={} on broadcaster={}", groupId, receiverList, name);
            for (Receiver<byte[]> receiver : receiverList) {
                receiver.onEviction();
            }
        }
    }

    @Override
    public void subscribe(String groupId, Receiver<byte[]> receiver) {
        List<Receiver<byte[]>> receiverList = handlerMap.get(groupId);
        receiverList.add(receiver);
        LOG.info("Subscribed receiverId={} to groupId={} on broadcaster {} with new handlerList={}", receiver, groupId, name, receiverList);
    }

    @Override
    public void unsubscribe(String groupId, String receiverId) {
        List<Receiver<byte[]>> receiverList = handlerMap.get(groupId);
        if (receiverList.removeIf((handler) -> handler.getId().equals(receiverId))) {
            LOG.info("Unsubscribed receiverId={} from groupId={} on broadcaster {} with new handlerList={}", receiverId, groupId, name, receiverList);
        }
    }

    @Override
    public void broadcast(String groupId, byte[] content) {
        List<Receiver<byte[]>> receiverList = handlerMap.get(groupId);
        if (receiverList == null) {
            LOG.info("Broadcast local to id={} on broadcaster {}, but there were no subscribers", groupId, name);
            return;
        }
        receiverList.removeIf(Receiver::isClosed);
        receiverList.forEach((handler) -> handler.onMessage(content));
        LOG.info("Broadcast local to id={}, receiverList={} on broadcaster {}", groupId, receiverList, name);
    }
}
