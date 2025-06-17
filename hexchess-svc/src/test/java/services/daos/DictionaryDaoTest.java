package services.daos;

import chess.ChessBoard;
import models.common.ColorSelect;
import models.state.ChessRoom;
import models.state.Player;
import models.entities.RankedEntity;
import models.common.TimeControl;
import models.views.ChessView;
import org.junit.ClassRule;
import org.junit.jupiter.api.*;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import redis.clients.jedis.JedisPooled;

import java.util.List;

@Testcontainers
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class DictionaryDaoTest {

    public static final DockerImageName REDIS_IMAGE = DockerImageName.parse("redis:6-alpine");

    @ClassRule
    public static GenericContainer<?> redis = new GenericContainer<>(REDIS_IMAGE)
            .withExposedPorts(6379);

    private JedisPooled jedis;
    private DictionaryDao dictionaryDao;

    @BeforeAll
    public void beforeAll() {
        redis.start();

        String host = redis.getHost();
        int port = redis.getFirstMappedPort();

        jedis = new JedisPooled(host, port);
        dictionaryDao = new DictionaryDao(jedis);
    }

    @BeforeEach
    public void beforeEach() {
        jedis.flushAll();
    }

    @AfterAll
    public void afterAll() {
        redis.stop();
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
    public void testSetThenGetViews() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";
        String id4 = "test-id4";

        ChessRoom room1 = ChessRoom.startWithGame(id1, TimeControl.REAL_TIME);
        ChessRoom room2 = ChessRoom.startWithGame(id2, TimeControl.REAL_TIME);
        ChessRoom room3 = ChessRoom.startWithGame(id3, TimeControl.REAL_TIME);
        ChessRoom room4 = ChessRoom.startWithGame(id4, TimeControl.REAL_TIME);

        // when
        dictionaryDao.setRoom(id1, room1);
        dictionaryDao.setRoom(id2, room2);
        dictionaryDao.setRoom(id3, room3);
        dictionaryDao.setRoom(id4, room4);

        List<ChessView> viewsList1 = dictionaryDao.getChessViews(1, 2);
        List<ChessView> viewsList2 = dictionaryDao.getChessViews(2, 2);

        // then
        List<ChessView> expectedViewList1 = List.of(
            new ChessView("test-id4", null, null, false, ColorSelect.RANDOM, TimeControl.REAL_TIME),
            new ChessView("test-id3", null, null, false, ColorSelect.RANDOM, TimeControl.REAL_TIME));
        List<ChessView> expectedViewList2 = List.of(
            new ChessView("test-id2", null, null, false, ColorSelect.RANDOM, TimeControl.REAL_TIME),
            new ChessView("test-id1", null, null, false, ColorSelect.RANDOM, TimeControl.REAL_TIME));

        Assertions.assertEquals(expectedViewList1, viewsList1);
        Assertions.assertEquals(expectedViewList2, viewsList2);
    }
    @Test
    public void testSetThenGetUserViews() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";

        ChessRoom room1 = ChessRoom.startWithGame(id1, TimeControl.REAL_TIME);
        ChessRoom room2 = ChessRoom.startWithGame(id2, TimeControl.REAL_TIME);
        ChessRoom room3 = ChessRoom.startWithGame(id2, TimeControl.REAL_TIME);

        room1.setWhitePlayer(new Player(1L));
        room1.setBlackPlayer(new Player(2L));

        room2.setBlackPlayer(new Player(1L));

        // when
        dictionaryDao.setRoom(id1, room1);
        dictionaryDao.setRoom(id2, room2);
        dictionaryDao.setRoom(id3, room3);

        List<ChessView> viewsList1 = dictionaryDao.getUserChessViews(1L);
        List<ChessView> viewsList2 = dictionaryDao.getUserChessViews(2L);
        List<ChessView> viewsList3 = dictionaryDao.getUserChessViews(3L);

        // then
        List<ChessView> expectedViewList1 = List.of(
            new ChessView("test-id2", null, new Player(1L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME),
            new ChessView("test-id1", new Player(1L), new Player(2L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME));
        List<ChessView> expectedViewList2 = List.of(
            new ChessView("test-id1", new Player(1L), new Player(2L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME));

        Assertions.assertEquals(expectedViewList1, viewsList1);
        Assertions.assertEquals(expectedViewList2, viewsList2);
        Assertions.assertEquals(List.of(), viewsList3);
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        Player player1 = new Player(1L, "test-name1", "", 0f);
        Player player2 = new Player(2L, "test-name2", "", 0f);

        // when
        dictionaryDao.setSession("session1", player1, 100);
        dictionaryDao.setSession("session2", player2, 1);

        Player player3 = dictionaryDao.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        Player player4 = dictionaryDao.getSession("session2");

        // then
        Assertions.assertEquals(player1, player3);
        Assertions.assertNull(player4);
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
