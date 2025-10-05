package services;

import redis.clients.jedis.ConnectionPoolConfig;
import redis.clients.jedis.Jedis;
import redis.clients.jedis.JedisPooled;
import redis.clients.jedis.JedisPubSub;

import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;

import static utils.Globals.*;

public interface SingleBroadcaster {
    String USERS_COUNT_TOPIC = "USERS_COUNT";
    String GAME_COUNT_TOPIC = "GAMES_COUNT";

    void subscribe(BroadcastReceiver<String> receiver);

    void unsubscribe(String handlerId);

    void broadcast(String content);

    class Global implements SingleBroadcaster {

        private final JedisPooled jedisPublisher;
        private final Jedis jedisSubscriber;
        private final String channel;
        private final Local localBroadcaster;

        public Global(ConnectionPoolConfig poolConfig, String host, int port, String channel) {
            this.jedisPublisher = new JedisPooled(poolConfig, host, port);
            this.jedisSubscriber = new Jedis(host, port);
            this.channel = channel;
            this.localBroadcaster = new Local(channel);
        }

        @Override
        public void subscribe(BroadcastReceiver<String> receiver) {
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

        public JedisPubSub startListenSubscribe() throws Exception {
            CompletableFuture<JedisPubSub> fut = new CompletableFuture<>();
            TP.execute(() -> startListenSubscribe(fut));
            return fut.get(MAX_WAIT_MS, TimeUnit.MILLISECONDS);
        }
    }

    class Local implements SingleBroadcaster {
        private final String name;
        private final Map<String, BroadcastReceiver<String>> receiverMap = new ConcurrentHashMap<>();

        public Local(String name) {
            this.name = name;
        }

        @Override
        public void subscribe(BroadcastReceiver<String> receiver) {
            receiverMap.put(receiver.getId(), receiver);
            LOG.info("Subscribed receiver={} on broadcaster {}", receiver, name);
        }

        @Override
        public void unsubscribe(String receiverId) {
            BroadcastReceiver<String> receiver = receiverMap.remove(receiverId);
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
}
