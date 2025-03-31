package services;

import io.jooby.WebSocket;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

import static utils.Globals.LOGGER;

public class GlobalBroadcaster implements Broadcaster {

    private static final String CHANNEL_NAME = "GLOBAL-BROADCAST";
    private static final char FIELD_SPLIT = 0x1e;

    private final JedisPooled jedis;
    private final LocalBroadcaster localBroadcaster = new LocalBroadcaster();

    public GlobalBroadcaster(JedisPooled jedis) {
        this.jedis = jedis;
    }

    @Override
    public void subscribe(String id, WebSocket ws) {
        localBroadcaster.subscribe(id, ws);
    }

    @Override
    public void unsubscribe(String id, WebSocket ws) {
        localBroadcaster.unsubscribe(id, ws);
    }

    @Override
    public void broadcast(String id, String content) {
        String message = id + FIELD_SPLIT + content;
        jedis.publish(CHANNEL_NAME, message);
        LOGGER.info("Broadcast global to id={}, message={}", id, message);
    }

    public JedisPubSub startListenSubscribe() throws ExecutionException, InterruptedException {
        CompletableFuture<JedisPubSub> futureSubscriber = new CompletableFuture<>();
        Thread.ofVirtual().start(() -> {
            var subscriber = new JedisPubSub() {
                @Override
                public void onSubscribe(String channel, int subscribedChannels) {
                    super.onSubscribe(channel, subscribedChannels);
                    LOGGER.info("Started the subscriber listener on channel={} for broadcast instance: {}", channel, this);
                    futureSubscriber.complete(this);
                }

                @Override
                public void onMessage(String channel, String message) {
                    try {
                        super.onMessage(channel, message);
                        int index = message.indexOf(FIELD_SPLIT);
                        if (index == -1) {
                            LOGGER.error("Invalid message format: {}", message);
                            return;
                        }
                        String id = message.substring(0, index);
                        String content = message.substring(index + 1);
                        LOGGER.info("Received a message on channel id={}, content{}, ref={}", id, content, this);
                        localBroadcaster.broadcast(id, content);
                    } catch (Exception ex) {
                        LOGGER.error("Error occurred in subscriber thread {}", String.valueOf(ex));
                    }
                }
            };
            jedis.subscribe(subscriber, CHANNEL_NAME); // start the subscriber, blocking the current thread until subscriber is stopped
        });

        // don't actually return the jedis subscriber until the thread notifies us that we've created it
        return futureSubscriber.get();
    }
}
