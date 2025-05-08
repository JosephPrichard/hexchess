package services.broadcast;

import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.function.Consumer;

import static utils.Globals.LOGGER;

public class GlobalBroadcaster implements Broadcaster {

    private static final char FIELD_SPLIT = 0x1e;

    private final JedisPooled jedisPublisher;
    private final Jedis jedisSubscriber;
    private final String channel;
    private final LocalBroadcaster localBroadcaster;

    public GlobalBroadcaster(String host, int port, String channel) {
        this.jedisPublisher = new JedisPooled(host, port);
        this.jedisSubscriber = new Jedis(host, port);
        this.channel = channel;
        this.localBroadcaster = new LocalBroadcaster(channel);
    }

    @Override
    public void subscribe(String groupId, String handlerId, Consumer<String> consumer) {
        localBroadcaster.subscribe(groupId, handlerId, consumer);
    }

    @Override
    public void unsubscribe(String groupId, String handlerId) {
        localBroadcaster.unsubscribe(groupId, handlerId);
    }

    @Override
    public void broadcast(String groupId, String content) {
        String message = groupId + FIELD_SPLIT + content;
        jedisPublisher.publish(channel, message);
        LOGGER.info("Broadcast global to id={}", groupId);
    }

    public JedisPubSub startListenSubscribe() throws ExecutionException, InterruptedException {
        CompletableFuture<JedisPubSub> futureSubscriber = new CompletableFuture<>();
        Thread.ofVirtual().start(() -> {
            try {
                JedisPubSub subscriber = new JedisPubSub() {
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
                            LOGGER.info("Received a message on channel id={}", id);

                            localBroadcaster.broadcast(id, content);
                        } catch (Exception ex) {
                            LOGGER.error("Error occurred in subscriber thread {}", String.valueOf(ex));
                        }
                    }
                };
                jedisSubscriber.subscribe(subscriber, channel); // start the subscriber, blocking the current thread until subscriber is stopped
            } catch (Exception ex) {
                futureSubscriber.completeExceptionally(ex);
            }
        });

        // don't actually return the jedis subscriber until the thread notifies us that we've created it
        return futureSubscriber.get();
    }
}