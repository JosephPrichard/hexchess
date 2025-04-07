package services;

import chess.ChessBoard;
import models.GameState;
import models.PlayerEntity;
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
        String id = "test-id";
        remoteDict.setGame(id, GameState.startWithGame(id));

        GameState firstGame = remoteDict.getGame(id);
        Assertions.assertEquals(GameState.startWithGame(id), firstGame);

        firstGame.getGame().getBoard().setPiece("a1", ChessBoard.BLACK_QUEEN);
        remoteDict.setGame(id, firstGame);

        GameState secondGame = remoteDict.getGame(id);

        Assertions.assertEquals(firstGame, secondGame);
    }

    @Test
    public void testGameScan() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";
        String id4 = "test-id4";

        PlayerEntity player1 = new PlayerEntity(1L, "name1");
        PlayerEntity player2 = new PlayerEntity(2L, "name2");
        PlayerEntity player3 = new PlayerEntity(3L, "name3");

        GameState game1 = GameState.ofPlayers(id1, player1, player2);
        GameState game2 = GameState.ofPlayers(id2, player2, player3);
        GameState game3 = GameState.ofPlayers(id3, player3, player1);
        GameState game4 = GameState.ofPlayers(id4, player2, player1);

        // when
        remoteDict.setGame(id1, game1);
        remoteDict.setGame(id2, game2);
        remoteDict.setGame(id3, game3);
        remoteDict.setGame(id4, game4);

        RemoteDict.GetGamesResult scanResult1 = remoteDict.getGames(null, 2);
        RemoteDict.GetGamesResult scanResult2 = remoteDict.getGames(scanResult1.getNextCursor(), 2);

        // then
        Assertions.assertEquals(2, scanResult1.getGameStates().size());
        Assertions.assertEquals(2, scanResult2.getGameStates().size());

        Assertions.assertEquals(id1, scanResult1.getGameStates().get(0).getId());
        Assertions.assertEquals(id2, scanResult1.getGameStates().get(1).getId());
        Assertions.assertEquals(id3, scanResult2.getGameStates().get(0).getId());
        Assertions.assertEquals(id4, scanResult2.getGameStates().get(1).getId());

        Assertions.assertNull(scanResult2.getNextCursor());
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        PlayerEntity player1 = new PlayerEntity(1L, "test-name1");
        PlayerEntity player2 = new PlayerEntity(2L, "test-name2");

        // when
        remoteDict.setSession("session1", player1, 100);
        remoteDict.setSession("session2", player2, 1);

        PlayerEntity actualPlayer1 = remoteDict.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        PlayerEntity actualPlayer2 = remoteDict.getSession("session2");

        // then
        Assertions.assertEquals(player1, actualPlayer1);
        Assertions.assertNull(actualPlayer2);
    }

    @Test
    public void testLeaderboard() {
        remoteDict.incrLeaderboardUser(10, 1500);
        remoteDict.incrLeaderboardUser(20, 1000);
        remoteDict.incrLeaderboardUser(30, 950);
        remoteDict.incrLeaderboardUser(40, 835);

        int rank1 = remoteDict.getLeaderboardRank(10);
        int rank2 = remoteDict.getLeaderboardRank(20);
        int rank3 = remoteDict.getLeaderboardRank(30);
        int rank4 = remoteDict.getLeaderboardRank(40);

        RemoteDict.Leaderboard leaderboard1 = remoteDict.getLeaderboard(0, 4);

        remoteDict.incrLeaderboardUser(new RemoteDict.EloChangeSet(20, 30));
        RemoteDict.Leaderboard leaderboard2 = remoteDict.getLeaderboard(1, 2);

        Assertions.assertEquals(1, rank1);
        Assertions.assertEquals(2, rank2);
        Assertions.assertEquals(3, rank3);
        Assertions.assertEquals(4, rank4);

        RemoteDict.Leaderboard expectedLeaderboard1 = new RemoteDict.Leaderboard(
            List.of(
                new RankedUser(10, 1),
                new RankedUser(20, 2),
                new RankedUser(30, 3),
                new RankedUser(40, 4)),
            1);
        Assertions.assertEquals(expectedLeaderboard1, leaderboard1);

        RemoteDict.Leaderboard expectedLeaderboard2 = new RemoteDict.Leaderboard(
            List.of(
                new RankedUser(20, 2),
                new RankedUser(30, 3)),
            2);
        Assertions.assertEquals(expectedLeaderboard2, leaderboard2);
    }
}
