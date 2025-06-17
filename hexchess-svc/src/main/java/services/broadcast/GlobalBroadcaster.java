package services.broadcast;

import redis.clients.jedis.ConnectionPoolConfig;
import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.nio.ByteBuffer;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

import static utils.Globals.EXECUTOR;
import static utils.Globals.LOG;

public class GlobalBroadcaster implements Broadcaster {

    private static final byte FIELD_SPLIT = 0x1e;

    private final JedisPooled jedisPublisher;
    private final Jedis jedisSubscriber;
    private final String channel;
    private final byte[] channelBytes;
    private final LocalBroadcaster localBroadcaster;

    public GlobalBroadcaster(ConnectionPoolConfig poolConfig, String host, int port, String channel) {
        this.jedisPublisher = new JedisPooled(poolConfig, host, port);
        this.jedisSubscriber = new Jedis(host, port);
        this.channel = channel;
        this.channelBytes = channel.getBytes();
        this.localBroadcaster = new LocalBroadcaster(channel);
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
        LOG.info("Broadcast global to id={}", groupId);
    }

    public JedisPubSub startListenSubscribe() throws ExecutionException, InterruptedException {
        CompletableFuture<JedisPubSub> futureSubscriber = new CompletableFuture<>();
        EXECUTOR.execute(() -> {
            try {
                JedisPubSub subscriber = new JedisPubSub() {
                    @Override
                    public void onSubscribe(String channel, int subscribedChannels) {
                        super.onSubscribe(channel, subscribedChannels);
                        LOG.info("Started the subscriber listener on channel={} for broadcast instance: {}", channel, this);
                        futureSubscriber.complete(this);
                    }

                    @Override
                    public void onMessage(String channel, String message) {
                        try {
                            super.onMessage(channel, message);
                            int index = message.indexOf(FIELD_SPLIT);
                            if (index == -1) {
                                LOG.error("Invalid message format: {}", message);
                                return;
                            }
                            String id = message.substring(0, index);
                            byte[] content = message.substring(index + 1).getBytes();
                            LOG.info("Received a message on channel id={}", id);

                            localBroadcaster.broadcast(id, content);
                        } catch (Exception ex) {
                            LOG.error("Error occurred in subscriber thread {}", String.valueOf(ex));
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