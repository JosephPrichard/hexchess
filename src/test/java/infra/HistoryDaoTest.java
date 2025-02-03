package infra;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.HistoryEntity;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.util.List;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class HistoryDaoTest {

    public EmbeddedPostgres pg;
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
        historyDao.insert("id1", "id2", HistoryEntity.WHITE_WIN, 30, -30, "{}");
        historyDao.insert("id2", "id3", HistoryEntity.BLACK_WIN, 30, -30, "{}");
        historyDao.insert("id3", "id1", HistoryEntity.DRAW, 30, -30, "{}");

        var actualHistory1 = historyDao.getHistory(1);
        var actualHistory2 = historyDao.getHistory(2);
        var actualHistory3 = historyDao.getHistory(3);

        // then
        var expectedHistory1 = new HistoryEntity(1, "id1", "id2", "user1", "user2",
            "us", "us", "{}", HistoryEntity.WHITE_WIN, 30, -30, null);
        var expectedHistory2 = new HistoryEntity(2, "id2", "id3", "user2", "user3",
            "us", "us", "{}", HistoryEntity.BLACK_WIN, 30, -30, null);
        var expectedHistory3 = new HistoryEntity(3, "id3", "id1", "user3", "user1",
            "us", "us", "{}", HistoryEntity.DRAW, 30, -30, null);

        Assertions.assertEquals(expectedHistory1, actualHistory1);
        Assertions.assertEquals(expectedHistory2, actualHistory2);
        Assertions.assertEquals(expectedHistory3, actualHistory3);
    }

    @Test
    public void testGetUserHistories() {
        // given
        createTestUserData(userDao);

        // when
        historyDao.insert("id1", "id2", HistoryEntity.WHITE_WIN, 30, -30, "{}");
        historyDao.insert("id2", "id3", HistoryEntity.BLACK_WIN, 30, -30, "{}");
        historyDao.insert("id3", "id1", HistoryEntity.DRAW, 30, -30, "{}");

        var actualHistoryList1 = historyDao.getUserHistories("id1", null, 5);
        var actualHistoryList2 = historyDao.getUserHistories("id1", 3L, 5);

        // then
        var expectedHistoryList1 = List.of(
            new HistoryEntity(3, "id3", "id1", "user3", "user1", "us", "us",
                null, HistoryEntity.DRAW, 30, -30, null),
            new HistoryEntity(1, "id1", "id2", "user1", "user2", "us", "us",
                null, HistoryEntity.WHITE_WIN, 30, -30, null));
        var expectedHistoryList2 = List.of(
            new HistoryEntity(1, "id1", "id2", "user1", "user2", "us", "us",
                null, HistoryEntity.WHITE_WIN, 30, -30, null));

        Assertions.assertEquals(expectedHistoryList1, actualHistoryList1);
        Assertions.assertEquals(expectedHistoryList2, actualHistoryList2);
    }
}
