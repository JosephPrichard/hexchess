package services;

import chess.ChessBoard;
import models.state.GameState;
import models.entities.PlayerEntity;
import models.entities.RankedEntity;
import models.common.TimeControl;
import org.junit.jupiter.api.*;
import redis.clients.jedis.JedisPooled;
import redis.embedded.RedisServer;
import services.daos.DictionaryDao;

import java.util.List;

import static utils.Globals.LOGGER;

// this is an integration test that runs against a real redis instance
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class DictionaryDaoTest {

    private RedisServer redisServer;
    private JedisPooled jedis;
    private DictionaryDao dictionaryDao;

    @BeforeAll
    public void beforeAll() {
        redisServer = new RedisServer(7777);
        try {
            redisServer.start();
        } catch (RuntimeException ex) {
            LOGGER.info("Redis instance is already started");
        }
        jedis = new JedisPooled("localhost", 7777);
        dictionaryDao = new DictionaryDao(jedis);
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
        dictionaryDao.setGame(id, GameState.startWithGame(id, TimeControl.UNLIMITED));

        GameState firstGame = dictionaryDao.getGame(id);
        Assertions.assertEquals(GameState.startWithGame(id, TimeControl.UNLIMITED), firstGame);

        firstGame.game.getBoard().setPiece("a1", ChessBoard.BLACK_QUEEN);
        dictionaryDao.setGame(id, firstGame);

        GameState secondGame = dictionaryDao.getGame(id);

        Assertions.assertEquals(firstGame, secondGame);
    }

    @Test
    public void testGameScan() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";
        String id4 = "test-id4";

        PlayerEntity player1 = new PlayerEntity(1L, "name1", null, null);
        PlayerEntity player2 = new PlayerEntity(2L, "name2", null, null);
        PlayerEntity player3 = new PlayerEntity(3L, "name3", null, null);

        GameState game1 = GameState.ofPlayers(id1, player1, player2);
        GameState game2 = GameState.ofPlayers(id2, player2, player3);
        GameState game3 = GameState.ofPlayers(id3, player3, player1);
        GameState game4 = GameState.ofPlayers(id4, player2, player1);

        // when
        dictionaryDao.setGame(id1, game1);
        dictionaryDao.setGame(id2, game2);
        dictionaryDao.setGame(id3, game3);
        dictionaryDao.setGame(id4, game4);

        List<GameState> gameStates1 = dictionaryDao.getGames(1, 2);
        List<GameState> gameStates2 = dictionaryDao.getGames(2, 2);

        // then
        Assertions.assertEquals(2, gameStates1.size());
        Assertions.assertEquals(2, gameStates2.size());

        Assertions.assertEquals(id1, gameStates1.get(0).id);
        Assertions.assertEquals(id2, gameStates1.get(1).id);
        Assertions.assertEquals(id3, gameStates2.get(0).id);
        Assertions.assertEquals(id4, gameStates2.get(1).id);
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        PlayerEntity player1 = new PlayerEntity(1L, "test-name1", null, null);
        PlayerEntity player2 = new PlayerEntity(2L, "test-name2", null, null);

        // when
        dictionaryDao.setSession("session1", player1, 100);
        dictionaryDao.setSession("session2", player2, 1);

        PlayerEntity actualPlayer1 = dictionaryDao.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        PlayerEntity actualPlayer2 = dictionaryDao.getSession("session2");

        // then
        Assertions.assertEquals(player1, actualPlayer1);
        Assertions.assertNull(actualPlayer2);
    }

    @Test
    public void testLeaderboard() {
        dictionaryDao.incrLeaderboardUser(10, 1500);
        dictionaryDao.incrLeaderboardUser(20, 1000);
        dictionaryDao.incrLeaderboardUser(30, 950);
        dictionaryDao.incrLeaderboardUser(40, 835);

        int rank1 = dictionaryDao.getLeaderboardRank(10);
        int rank2 = dictionaryDao.getLeaderboardRank(20);
        int rank3 = dictionaryDao.getLeaderboardRank(30);
        int rank4 = dictionaryDao.getLeaderboardRank(40);

        DictionaryDao.Leaderboard leaderboard1 = dictionaryDao.getLeaderboard(0, 4);

        dictionaryDao.incrLeaderboardUser(new DictionaryDao.EloChangeSet(20, 30));
        DictionaryDao.Leaderboard leaderboard2 = dictionaryDao.getLeaderboard(1, 2);

        Assertions.assertEquals(1, rank1);
        Assertions.assertEquals(2, rank2);
        Assertions.assertEquals(3, rank3);
        Assertions.assertEquals(4, rank4);

        DictionaryDao.Leaderboard expectedLeaderboard1 = new DictionaryDao.Leaderboard(
            List.of(
                new RankedEntity(10, 1),
                new RankedEntity(20, 2),
                new RankedEntity(30, 3),
                new RankedEntity(40, 4)),
            1);
        Assertions.assertEquals(expectedLeaderboard1, leaderboard1);

        DictionaryDao.Leaderboard expectedLeaderboard2 = new DictionaryDao.Leaderboard(
            List.of(
                new RankedEntity(20, 2),
                new RankedEntity(30, 3)),
            2);
        Assertions.assertEquals(expectedLeaderboard2, leaderboard2);
    }
}
