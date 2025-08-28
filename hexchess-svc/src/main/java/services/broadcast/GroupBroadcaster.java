package services.broadcast;

import com.github.benmanes.caffeine.cache.Caffeine;
import com.github.benmanes.caffeine.cache.LoadingCache;
import com.github.benmanes.caffeine.cache.RemovalCause;
import com.github.benmanes.caffeine.cache.Scheduler;
import org.checkerframework.checker.nullness.qual.Nullable;
import redis.clients.jedis.ConnectionPoolConfig;
import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.nio.ByteBuffer;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;

import static utils.Globals.*;

public interface GroupBroadcaster {
    String GAMES_TOPIC = "GAMES";
    String USERS_TOPIC = "NOTIFICATIONS";

    void subscribe(String groupId, BroadcastReceiver<byte[]> receiver);

    void unsubscribe(String groupId, String handlerId);

    void broadcast(String groupId, byte[] content);

    class Global implements GroupBroadcaster {

        private static final byte FIELD_SPLIT = 0x1e;

        private final JedisPooled jedisPublisher;
        private final Jedis jedisSubscriber;
        private final String channel;
        private final byte[] channelBytes;
        private final Local localBroadcaster;

        public Global(ConnectionPoolConfig poolConfig, String host, int port, String channel) {
            this.jedisPublisher = new JedisPooled(poolConfig, host, port);
            this.jedisSubscriber = new Jedis(host, port);
            this.channel = channel;
            this.channelBytes = channel.getBytes();
            this.localBroadcaster = new Local(channel);
        }

        @Override
        public void subscribe(String groupId, BroadcastReceiver<byte[]> receiver) {
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

    class Local implements GroupBroadcaster {
        private final String name;
        private final LoadingCache<String, List<BroadcastReceiver<byte[]>>> handlerMap = Caffeine.newBuilder()
            .scheduler(Scheduler.systemScheduler())
            .evictionListener(this::handleEviction)
            .expireAfterAccess(1, TimeUnit.HOURS)
            .build(key -> new CopyOnWriteArrayList<>());

        public Local(String name) {
            this.name = name;
        }

        private void handleEviction(@Nullable String groupId, @Nullable List<BroadcastReceiver<byte[]>> receiverList, RemovalCause cause) {
            if (receiverList != null) {
                LOG.info("Evicted groupId={} with receiverList={} on broadcaster={}", groupId, receiverList, name);
                for (BroadcastReceiver<byte[]> receiver : receiverList) {
                    receiver.onEviction();
                }
            }
        }

        @Override
        public void subscribe(String groupId, BroadcastReceiver<byte[]> receiver) {
            List<BroadcastReceiver<byte[]>> receiverList = handlerMap.get(groupId);
            receiverList.add(receiver);
            LOG.info("Subscribed receiverId={} to groupId={} on broadcaster {} with new handlerList={}", receiver, groupId, name, receiverList);
        }

        @Override
        public void unsubscribe(String groupId, String receiverId) {
            List<BroadcastReceiver<byte[]>> receiverList = handlerMap.get(groupId);
            if (receiverList.removeIf((handler) -> handler.getId().equals(receiverId))) {
                LOG.info("Unsubscribed receiverId={} from groupId={} on broadcaster {} with new handlerList={}", receiverId, groupId, name, receiverList);
            }
        }

        @Override
        public void broadcast(String groupId, byte[] content) {
            List<BroadcastReceiver<byte[]>> receiverList = handlerMap.get(groupId);
            if (receiverList == null) {
                LOG.info("Broadcast local to id={} on broadcaster {}, but there were no subscribers", groupId, name);
                return;
            }
            receiverList.removeIf(BroadcastReceiver::isClosed);
            receiverList.forEach((handler) -> handler.onMessage(content));
            LOG.info("Broadcast local to id={}, receiverList={} on broadcaster {}", groupId, receiverList, name);
        }
    }
}
