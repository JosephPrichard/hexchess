package services.broadcast;

import redis.clients.jedis.ConnectionPoolConfig;
import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.nio.ByteBuffer;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

import static utils.Globals.*;

public class GlobalGroupBroadcaster implements GroupBroadcaster {

    private static final byte FIELD_SPLIT = 0x1e;

    private final JedisPooled jedisPublisher;
    private final Jedis jedisSubscriber;
    private final String channel;
    private final byte[] channelBytes;
    private final LocalGroupBroadcaster localBroadcaster;

    public GlobalGroupBroadcaster(ConnectionPoolConfig poolConfig, String host, int port, String channel) {
        this.jedisPublisher = new JedisPooled(poolConfig, host, port);
        this.jedisSubscriber = new Jedis(host, port);
        this.channel = channel;
        this.channelBytes = channel.getBytes();
        this.localBroadcaster = new LocalGroupBroadcaster(channel);
    }

    @Override
    public void subscribe(String groupId, Receiver<byte[]> receiver) {
        localBroadcaster.subscribe(groupId, receiver);
    }

    @Override
    public void unsubscribe(String groupId, String receiverId) {
        localBroadcaster.unsubscribe(groupId, receiverId);
    }

    @Override
    public void broadcast(String groupId, byte[] content) {
        ByteBuffer bytes = ByteBuffer.allocate(groupId.length() + 1 + content.length);
        bytes.put(groupId.getBytes());
        bytes.put(FIELD_SPLIT);
        bytes.put(content);

        jedisPublisher.publish(channelBytes, bytes.array());
        LOG.info("Broadcast content={} global for broadcaster {} to id={}", content, channel, groupId);
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
                        int index = message.indexOf(FIELD_SPLIT);
                        if (index == -1) {
                            LOG.error("Invalid message format in subscriber thread: {}", message);
                            return;
                        }
                        String id = message.substring(0, index);
                        byte[] content = message.substring(index + 1).getBytes();
                        LOG.info("Received a message on channel {} id={}", channel, id);

                        localBroadcaster.broadcast(id, content);
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

    public JedisPubSub startListenSubscribe() throws Exception {
        CompletableFuture<JedisPubSub> fut = new CompletableFuture<>();
        EXECUTOR.execute(() -> startListenSubscribe(fut));
        return fut.get(MAX_WAIT_MS, TimeUnit.MILLISECONDS);
    }
}