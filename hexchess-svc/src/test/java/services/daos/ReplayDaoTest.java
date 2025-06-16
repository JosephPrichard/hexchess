package services.daos;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.entities.ReplayEntity;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.util.List;

import static mocks.ReplayMocks.*;
import static services.daos.UserDao.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class ReplayDaoTest {

    private EmbeddedPostgres pg;
    private DataSource ds;
    private UserDao userDao;
    private ReplayDao replayDao;

    @BeforeAll
    public void beforeAll() throws IOException {
        pg = EmbeddedPostgres.builder().start();
        ds = pg.getPostgresDatabase();

        userDao = new UserDao(ds);
        replayDao = new ReplayDao(ds);
    }

    @BeforeEach
    public void beforeEach() {
        Config.createSchema(ds);
    }

    public static void createTestUserData(UserDao userDao) {
        userDao.insert(new UserInst("user1", "password1", "us", 0f, 0, 0));
        userDao.insert(new UserInst("user2", "password2", "us", 0f, 0, 0));
        userDao.insert(new UserInst("user3", "password3", "us", 0f, 0, 0));
        userDao.insert(new UserInst("user4", "password4", "us", 0f, 0, 0));
        userDao.insert(new UserInst("user5", "password5", "us", 0f, 0, 0));
    }

    @Test
    public void testInsertThenGet() {
        // given
        createTestUserData(userDao);

        // when
        replayDao.insert(1L, 2L, ReplayEntity.WHITE_WIN, ReplayEntity.CHECKMATE, 30, -30, "{}");
        replayDao.insert(2L, 3L, ReplayEntity.BLACK_WIN, ReplayEntity.CHECKMATE, 30, -30, "{}");
        replayDao.insert(3L, 1L, ReplayEntity.DRAW, ReplayEntity.CHECKMATE, 30, -30, "{}");

        ReplayEntity actualReplay1 = replayDao.getReplay(1);
        ReplayEntity actualReplay2 = replayDao.getReplay(2);
        ReplayEntity actualReplay3 = replayDao.getReplay(3);

        // then
        Assertions.assertEquals(REPLAY1, actualReplay1);
        Assertions.assertEquals(REPLAY2, actualReplay2);
        Assertions.assertEquals(REPLAY3, actualReplay3);
    }

    @Test
    public void testGetUserReplays() {
        // given
        createTestUserData(userDao);

        // when
        replayDao.insert(1L, 2L, ReplayEntity.WHITE_WIN, ReplayEntity.CHECKMATE, 30, -30, "{}");
        replayDao.insert(2L, 3L, ReplayEntity.BLACK_WIN, ReplayEntity.CHECKMATE, 30, -30, "{}");
        replayDao.insert(3L, 1L, ReplayEntity.DRAW, ReplayEntity.CHECKMATE, 30, -30, "{}");

        List<ReplayEntity> actualReplayList1 = replayDao.getUserReplays(1L, null, 5);
        List<ReplayEntity> actualReplayList2 = replayDao.getUserReplays(1L, 3L, 5);

        // then
        List<ReplayEntity> expectedReplayList1 = List.of(REPLAY3, REPLAY1);
        List<ReplayEntity> expectedReplayList2 = List.of(REPLAY1);

        Assertions.assertEquals(expectedReplayList1, actualReplayList1);
        Assertions.assertEquals(expectedReplayList2, actualReplayList2);
    }

    @Test
    public void testGetReplayMoveList() {
        // given
        createTestUserData(userDao);

        // when
        replayDao.insert(1L, 2L, ReplayEntity.WHITE_WIN, ReplayEntity.CHECKMATE, 30, -30, "[]");


        String actualMoveList = replayDao.getReplayMoveList(1L);

        // then
        Assertions.assertEquals("[]", actualMoveList);
    }
}
