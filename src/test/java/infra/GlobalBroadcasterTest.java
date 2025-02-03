package infra;

import io.jooby.WebSocket;
import org.junit.jupiter.api.*;
import redis.clients.jedis.JedisPooled;
import redis.embedded.RedisServer;

import java.util.concurrent.ExecutionException;

import static org.mockito.Mockito.*;
import static utils.Globals.LOGGER;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class GlobalBroadcasterTest {

    private RedisServer redisServer;
    private JedisPooled jedis;

    @BeforeAll
    public void beforeAll() {
        redisServer = new RedisServer(6379);
        try {
            redisServer.start();
        } catch (RuntimeException ex) {
            LOGGER.info("Redis instance is already started");
        }
        jedis = new JedisPooled("localhost", 6379);
    }

    @BeforeEach
    public void beforeEach() {
        jedis.flushAll();
    }

    @AfterAll
    public void afterAll() {
        redisServer.stop();
    }

    @Test
    public void testBroadcast() throws InterruptedException, ExecutionException {
        // given
        var broadcastService1 = new GlobalBroadcaster(jedis);
        var broadcastService2 = new GlobalBroadcaster(jedis);

        var mockWs1 = mock(WebSocket.class);
        var mockWs2 = mock(WebSocket.class);
        var mockWs3 = mock(WebSocket.class);

        var subscriber1 = broadcastService1.startListenSubscribe();
        var subscriber2 = broadcastService2.startListenSubscribe();

        // when
        broadcastService1.subscribe("id", mockWs1);
        broadcastService1.subscribe("id", mockWs2);
        broadcastService2.subscribe("id", mockWs3);

        broadcastService1.broadcast("id", "Test content 1");

        Thread.sleep(500); // wait til the previous messages have been delivered, before we unsubscribe
        broadcastService1.unsubscribe("id", mockWs2);

        broadcastService1.broadcast("id", "Test content 2");

        subscriber1.unsubscribe();
        subscriber2.unsubscribe();

        // then
        verify(mockWs1).send("Test content 1");
        verify(mockWs2).send("Test content 1");
        verify(mockWs3).send("Test content 1");

        verify(mockWs1).send("Test content 2");
        verify(mockWs2, times(0)).send("Test content 2");
        verify(mockWs3).send("Test content 2");
    }
}
