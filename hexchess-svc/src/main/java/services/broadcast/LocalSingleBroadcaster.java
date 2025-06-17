package services.broadcast;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

import static utils.Globals.LOG;

public class LocalSingleBroadcaster implements SingleBroadcaster {

    private final String name;
    private final Map<String, Receiver<String>> receiverMap = new ConcurrentHashMap<>();

    public LocalSingleBroadcaster(String name) {
        this.name = name;
    }

    @Override
    public void subscribe(Receiver<String> receiver) {
        receiverMap.put(receiver.getId(), receiver);
        LOG.info("Subscribed receiver={} on broadcaster {}", receiver, name);
    }

    @Override
    public void unsubscribe(String receiverId) {
        Receiver<String> receiver = receiverMap.remove(receiverId);
        if (receiver != null) {
            LOG.info("Unsubscribed receiverId={} on broadcaster {}", receiverId, name);
        }
    }

    @Override
    public void broadcast(String content) {
        receiverMap.entrySet().removeIf((entry) -> entry.getValue().isClosed());
        receiverMap.forEach((k, v) -> v.onMessage(content));
        LOG.info("Broadcasting content={} on broadcaster {}", content, name);
    }
}
