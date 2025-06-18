package services.broadcast;

import redis.clients.jedis.ConnectionPoolConfig;
import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

import static utils.Globals.EXECUTOR;
import static utils.Globals.LOG;

public class GlobalSingleBroadcaster implements SingleBroadcaster {

    private static final byte FIELD_SPLIT = 0x1e;

    private final JedisPooled jedisPublisher;
    private final Jedis jedisSubscriber;
    private final String channel;
    private final LocalSingleBroadcaster localBroadcaster;

    public GlobalSingleBroadcaster(ConnectionPoolConfig poolConfig, String host, int port, String channel) {
        this.jedisPublisher = new JedisPooled(poolConfig, host, port);
        this.jedisSubscriber = new Jedis(host, port);
        this.channel = channel;
        this.localBroadcaster = new LocalSingleBroadcaster(channel);
    }

    @Override
    public void subscribe(Receiver<String> receiver) {
        localBroadcaster.subscribe(receiver);
    }

    @Override
    public void unsubscribe(String receiverId) {
        localBroadcaster.unsubscribe(receiverId);
    }

    @Override
    public void broadcast(String content) {
        jedisPublisher.publish(channel, content);
        LOG.info("Broadcast content={} to broadcaster = {}", content, channel);
    }

    public void startListenSubscribe(CompletableFuture<JedisPubSub> fut) {
        try {
            JedisPubSub subscriber = new JedisPubSub() {
                @Override
                public void onSubscribe(String channel, int subscribedChannels) {
                    super.onSubscribe(channel, subscribedChannels);
                    LOG.info("Started the subscriber listener on channel={} for broadcast instance: {}", channel, this);
                    fut.complete(this);
                }

                @Override
                public void onMessage(String channel, String message) {
                    try {
                        super.onMessage(channel, message);
                        LOG.info("Received a message on channel={}", channel);
                        localBroadcaster.broadcast(message);
                    } catch (Exception ex) {
                        LOG.error("Error occurred in subscriber thread", ex);
                    }
                }
            };
            jedisSubscriber.subscribe(subscriber, channel); // start the subscriber, blocking the current thread until subscriber is stopped
        } catch (Exception ex) {
            fut.completeExceptionally(ex);
        }
    }

    public JedisPubSub startListenSubscribe() throws ExecutionException, InterruptedException {
        CompletableFuture<JedisPubSub> fut = new CompletableFuture<>();
        CompletableFuture.runAsync(() -> startListenSubscribe(fut), EXECUTOR);
        return fut.get();
    }
}