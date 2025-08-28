package services;

import chess.ChessBoard;
import models.enums.ColorSelect;
import models.state.ChessState;
import models.state.PlayerState;
import models.entities.UserRankEntity;
import models.enums.TimeControl;
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

        ChessState input = ChessState.startWithGame(id, TimeControl.UNLIMITED);
        dictionaryDao.setRoom(id, input);

        ChessState output1 = dictionaryDao.getRoom(id);
        Assertions.assertEquals(input, output1);

        output1.getGame().getBoard().setPiece("a1", ChessBoard.BLACK_QUEEN);
        dictionaryDao.setRoom(id, output1);

        ChessState output2 = dictionaryDao.getRoom(id);

        Assertions.assertEquals(output1, output2);
    }

    @Test
    public void testSetThenGetViews() {
        // given
        String id1 = "test-id1";
        String id2 = "test-id2";
        String id3 = "test-id3";
        String id4 = "test-id4";

        ChessState room1 = ChessState.startWithGame(id1, TimeControl.REAL_TIME);
        ChessState room2 = ChessState.startWithGame(id2, TimeControl.REAL_TIME);
        ChessState room3 = ChessState.startWithGame(id3, TimeControl.REAL_TIME);
        ChessState room4 = ChessState.startWithGame(id4, TimeControl.REAL_TIME);

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

        ChessState room1 = ChessState.startWithGame(id1, TimeControl.REAL_TIME);
        ChessState room2 = ChessState.startWithGame(id2, TimeControl.REAL_TIME);
        ChessState room3 = ChessState.startWithGame(id2, TimeControl.REAL_TIME);

        room1.setWhitePlayer(new PlayerState(1L));
        room1.setBlackPlayer(new PlayerState(2L));

        room2.setBlackPlayer(new PlayerState(1L));

        // when
        dictionaryDao.setRoom(id1, room1);
        dictionaryDao.setRoom(id2, room2);
        dictionaryDao.setRoom(id3, room3);

        List<ChessView> viewsList1 = dictionaryDao.getUserChessViews(1L);
        List<ChessView> viewsList2 = dictionaryDao.getUserChessViews(2L);
        List<ChessView> viewsList3 = dictionaryDao.getUserChessViews(3L);

        // then
        List<ChessView> expectedViewList1 = List.of(
            new ChessView("test-id2", null, new PlayerState(1L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME),
            new ChessView("test-id1", new PlayerState(1L), new PlayerState(2L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME));
        List<ChessView> expectedViewList2 = List.of(
            new ChessView("test-id1", new PlayerState(1L), new PlayerState(2L), false, ColorSelect.RANDOM, TimeControl.REAL_TIME));

        Assertions.assertEquals(expectedViewList1, viewsList1);
        Assertions.assertEquals(expectedViewList2, viewsList2);
        Assertions.assertEquals(List.of(), viewsList3);
    }

    @Test
    public void testSessions() throws InterruptedException {
        // given
        PlayerState player1 = new PlayerState(1L, "test-name1", "", 0f);
        PlayerState player2 = new PlayerState(2L, "test-name2", "", 0f);

        // when
        dictionaryDao.setSession("session1", player1, 100);
        dictionaryDao.setSession("session2", player2, 1);

        PlayerState player3 = dictionaryDao.getSession("session1");

        Thread.sleep(1000); // wait for key to expire
        PlayerState player4 = dictionaryDao.getSession("session2");

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
                new UserRankEntity(10, 1),
                new UserRankEntity(20, 2),
                new UserRankEntity(30, 3),
                new UserRankEntity(40, 4)),
            1);
        Assertions.assertEquals(expectedLeaderboard1, leaderboard1);

        DictionaryDao.Leaderboard expectedLeaderboard2 = new DictionaryDao.Leaderboard(
            List.of(
                new UserRankEntity(20, 2),
                new UserRankEntity(30, 3)),
            2);
        Assertions.assertEquals(expectedLeaderboard2, leaderboard2);
    }
}
