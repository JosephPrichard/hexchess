package dao;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.History;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.util.List;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class HistoryDaoTest {

    private EmbeddedPostgres pg;
    private DataSource ds;
    private UserDao userDao;
    private HistoryDao historyDao;

    @BeforeAll
    public void beforeAll() throws IOException {
        pg = EmbeddedPostgres.builder().start();
        ds = pg.getPostgresDatabase();

        userDao = new UserDao(ds);
        historyDao = new HistoryDao(ds);
    }

    @BeforeEach
    public void beforeEach() {
        Config.createSchema(ds);
    }

    public static void createTestUserData(UserDao userDao) {
        userDao.insert(new UserDao.UserInst("id1", "user1", "password1", "us", 0f, 0, 0));
        userDao.insert(new UserDao.UserInst("id2", "user2", "password2", "us", 0f, 0, 0));
        userDao.insert(new UserDao.UserInst("id3", "user3", "password3", "us", 0f, 0, 0));
        userDao.insert(new UserDao.UserInst("id4", "user4", "password4", "us", 0f, 0, 0));
        userDao.insert(new UserDao.UserInst("id5", "user5", "password5", "us", 0f, 0, 0));
    }

    @Test
    public void testInsertThenGet() {
        // given
        createTestUserData(userDao);

        // when
        historyDao.insert("id1", "id2", History.WHITE_WIN, 30, -30, "{}");
        historyDao.insert("id2", "id3", History.BLACK_WIN, 30, -30, "{}");
        historyDao.insert("id3", "id1", History.DRAW, 30, -30, "{}");

        var actualHistory1 = historyDao.getHistory(1);
        var actualHistory2 = historyDao.getHistory(2);
        var actualHistory3 = historyDao.getHistory(3);

        // then
        var expectedHistory1 = new History(1, "id1", "id2", "user1", "user2",
            "us", "us", "{}", History.WHITE_WIN, 30, -30, null);
        var expectedHistory2 = new History(2, "id2", "id3", "user2", "user3",
            "us", "us", "{}", History.BLACK_WIN, 30, -30, null);
        var expectedHistory3 = new History(3, "id3", "id1", "user3", "user1",
            "us", "us", "{}", History.DRAW, 30, -30, null);

        Assertions.assertEquals(expectedHistory1, actualHistory1);
        Assertions.assertEquals(expectedHistory2, actualHistory2);
        Assertions.assertEquals(expectedHistory3, actualHistory3);
    }

    @Test
    public void testGetUserHistories() {
        // given
        createTestUserData(userDao);

        // when
        historyDao.insert("id1", "id2", History.WHITE_WIN, 30, -30, "{}");
        historyDao.insert("id2", "id3", History.BLACK_WIN, 30, -30, "{}");
        historyDao.insert("id3", "id1", History.DRAW, 30, -30, "{}");

        var actualHistoryList1 = historyDao.getUserHistories("id1", null, 5);
        var actualHistoryList2 = historyDao.getUserHistories("id1", 3L, 5);

        // then
        var expectedHistoryList1 = List.of(
            new History(3, "id3", "id1", "user3", "user1", "us", "us",
                null, History.DRAW, 30, -30, null),
            new History(1, "id1", "id2", "user1", "user2", "us", "us",
                null, History.WHITE_WIN, 30, -30, null));
        var expectedHistoryList2 = List.of(
            new History(1, "id1", "id2", "user1", "user2", "us", "us",
                null, History.WHITE_WIN, 30, -30, null));

        Assertions.assertEquals(expectedHistoryList1, actualHistoryList1);
        Assertions.assertEquals(expectedHistoryList2, actualHistoryList2);
    }
}
