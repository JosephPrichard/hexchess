package services;

import chess.ChessBoard;
import models.state.ChessRoom;
import models.state.Player;
import models.entities.RankedEntity;
import models.common.TimeControl;
import org.junit.jupiter.api.*;
import redis.clients.jedis.JedisPooled;
import redis.embedded.RedisServer;
import services.daos.DictionaryDao;

import java.util.List;

import static utils.Globals.LOG;

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
            LOG.info("Redis instance is already started");
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

        ChessRoom input = ChessRoom.startWithGame(id, TimeControl.UNLIMITED);
        dictionaryDao.setRoom(id, input);

        ChessRoom output1 = dictionaryDao.getRoom(id);
        Assertions.assertEquals(input, output1);

        output1.getGame().getBoard().setPiece("a1", ChessBoard.BLACK_QUEEN);
        dictionaryDao.setRoom(id, output1);

        ChessRoom output2 = dictionaryDao.getRoom(id);

        Assertions.assertEquals(output1, output2);
    }

    @Test
    public void testGetSetRooms() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";
        String id4 = "test-id4";

        ChessRoom game1 = ChessRoom.startWithGame(id1, TimeControl.REAL_TIME);
        ChessRoom game2 = ChessRoom.startWithGame(id2, TimeControl.REAL_TIME);
        ChessRoom game3 = ChessRoom.startWithGame(id3, TimeControl.REAL_TIME);
        ChessRoom game4 = ChessRoom.startWithGame(id4, TimeControl.REAL_TIME);

        // when
        dictionaryDao.setRoom(id1, game1);
        dictionaryDao.setRoom(id2, game2);
        dictionaryDao.setRoom(id3, game3);
        dictionaryDao.setRoom(id4, game4);

        List<ChessRoom> gameStates1 = dictionaryDao.getRooms(1, 2);
        List<ChessRoom> gameStates2 = dictionaryDao.getRooms(2, 2);

        // then
        Assertions.assertEquals(2, gameStates1.size());
        Assertions.assertEquals(2, gameStates2.size());

        Assertions.assertEquals(id1, gameStates1.get(0).getId());
        Assertions.assertEquals(id2, gameStates1.get(1).getId());
        Assertions.assertEquals(id3, gameStates2.get(0).getId());
        Assertions.assertEquals(id4, gameStates2.get(1).getId());
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        Player player1 = new Player(1L, "test-name1", null, null);
        Player player2 = new Player(2L, "test-name2", null, null);

        // when
        dictionaryDao.setSession("session1", player1, 100);
        dictionaryDao.setSession("session2", player2, 1);

        Player actualPlayer1 = dictionaryDao.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        Player actualPlayer2 = dictionaryDao.getSession("session2");

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
