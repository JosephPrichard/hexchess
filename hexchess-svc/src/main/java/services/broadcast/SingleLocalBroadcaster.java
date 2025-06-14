package services.broadcast;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Consumer;

import static utils.Globals.LOG;

public class SingleLocalBroadcaster implements SingleBroadcaster {

    private final String name;
    private final Map<String, Handler<String>> handlerMap = new ConcurrentHashMap<>();

    public SingleLocalBroadcaster(String name) {
        this.name = name;
    }

    @Override
    public void subscribe(String handlerId, Consumer<String> consumer) {
        handlerMap.put(handlerId, new Handler<>(handlerId, consumer));
        LOG.info("Subscribed handlerId={} on broadcaster {}", handlerId, name);
    }

    @Override
    public void unsubscribe(String handlerId) {
        Handler<String> handler = handlerMap.remove(handlerId);
        if (handler != null) {
            LOG.info("Unsubscribed handlerId={} fon broadcaster {}", handlerId, name);
        }
    }

    @Override
    public void broadcast(String content) {
        handlerMap.forEach((k, v) -> v.getConsumer().accept(content));
        LOG.info("Broadcasting on broadcaster {}", name);
    }
}
