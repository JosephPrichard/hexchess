package services;

import domain.ChessBoard;
import models.GameState;
import models.Player;
import models.RankedUser;
import org.junit.jupiter.api.*;
import redis.clients.jedis.JedisPooled;
import redis.embedded.RedisServer;

import java.util.List;

import static utils.Globals.LOGGER;

// this is an integration test that runs against a real redis instance
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class RemoteDictTest {

    private RedisServer redisServer;
    private JedisPooled jedis;
    private RemoteDict remoteDict;

    @BeforeAll
    public void beforeAll() {
        redisServer = new RedisServer(7777);
        try {
            redisServer.start();
        } catch (RuntimeException ex) {
            LOGGER.info("Redis instance is already started");
        }
        jedis = new JedisPooled("localhost", 7777);
        remoteDict = new RemoteDict(jedis);
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
    public void testGameUpdate() {
        var id = "test-id";
        remoteDict.setGame(id, GameState.startWithGame(id));

        var firstGame = remoteDict.getGame(id);
        Assertions.assertEquals(GameState.startWithGame(id), firstGame);

        firstGame.getGame().getBoard().setPiece("a1", ChessBoard.BLACK_QUEEN);
        remoteDict.setGame(id, firstGame);

        var secondGame = remoteDict.getGame(id);

        Assertions.assertEquals(firstGame, secondGame);
    }

    @Test
    public void testGameScan() {
        // given
        var id1 = "test-id1";
        var id2 = "test-id2";
        var id3 = "test-id3";
        var id4 = "test-id4";

        var player1 = new Player("id1", "name1");
        var player2 = new Player("id2", "name2");
        var player3 = new Player("id3", "name3");

        var game1 = GameState.ofPlayers(player1, player2);
        var game2 = GameState.ofPlayers(player2, player3);
        var game3 = GameState.ofPlayers(player3, player1);
        var game4 = GameState.ofPlayers(player2, player1);

        // when
        remoteDict.setGame(id1, game1);
        remoteDict.setGame(id2, game2);
        remoteDict.setGame(id3, game3);
        remoteDict.setGame(id4, game4);

        var scanResult1 = remoteDict.getGames(null, 2);
        var scanResult2 = remoteDict.getGames(scanResult1.getNextCursor(), 2);

        // then
        Assertions.assertEquals(2, scanResult1.getGameStates().size());
        Assertions.assertEquals(2, scanResult2.getGameStates().size());
        Assertions.assertNull(scanResult2.getNextCursor());
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        var player1 = new Player("test-id1", "test-name1");
        var player2 = new Player("test-id2", "test-name2");

        // when
        remoteDict.setSession("session1", player1, 100);
        remoteDict.setSession("session2", player2, 1);

        var actualPlayer1 = remoteDict.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        var actualPlayer2 = remoteDict.getSession("session2");

        // then
        Assertions.assertEquals(player1, actualPlayer1);
        Assertions.assertNull(actualPlayer2);
    }

    @Test
    public void testLeaderboard() {
        remoteDict.incrLeaderboardUser("user1", 930);
        remoteDict.incrLeaderboardUser("user2", 995);
        remoteDict.incrLeaderboardUser("user3", 1000);
        remoteDict.incrLeaderboardUser("user4", 1500);

        int rank1 = remoteDict.getLeaderboardRank("user1");
        int rank2 = remoteDict.getLeaderboardRank("user2");
        int rank3 = remoteDict.getLeaderboardRank("user3");
        int rank4 = remoteDict.getLeaderboardRank("user4");

        var leaderboard1 = remoteDict.getLeaderboard(0, 4);

        remoteDict.incrLeaderboardUser(new RemoteDict.EloChangeSet("user2", 30));
        var leaderboard2 = remoteDict.getLeaderboard(1, 2);

        Assertions.assertEquals(1, rank1);
        Assertions.assertEquals(2, rank2);
        Assertions.assertEquals(3, rank3);
        Assertions.assertEquals(4, rank4);

        var expectedLeaderboard1 = new RemoteDict.Leaderboard(
            List.of(
                new RankedUser("user1", 1),
                new RankedUser("user2", 2),
                new RankedUser("user3", 3),
                new RankedUser("user4", 4)),
            1);
        Assertions.assertEquals(expectedLeaderboard1, leaderboard1);

        var expectedLeaderboard2 = new RemoteDict.Leaderboard(
            List.of(
                new RankedUser("user3", 2),
                new RankedUser("user2", 3)),
            2);
        Assertions.assertEquals(expectedLeaderboard2, leaderboard2);
    }
}
