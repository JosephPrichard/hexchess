package dao;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.ChallengeEntity;
import models.UserEntity;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.sql.Timestamp;
import java.time.Duration;
import java.util.List;

import static dao.UserDao.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class ChallengeDaoTest {

    private EmbeddedPostgres pg;
    private DataSource ds;
    private UserDao userDao;
    private ChallengeDao challengeDao;

    @BeforeAll
    public void beforeAll() throws IOException {
        pg = EmbeddedPostgres.builder().start();
        ds = pg.getPostgresDatabase();

        userDao = new UserDao(ds);
        challengeDao = new ChallengeDao(ds);
    }

    @BeforeEach
    public void beforeEach() {
        Config.createSchema(ds);
    }

    public static void createTestData(UserDao userDao) {
        userDao.insert(new UserInst("user1", "password1", "us", 1000f, 0, 0));
        userDao.insert(new UserInst("user2", "password2", "us", 1005f, 1, 0));
        userDao.insert(new UserInst("user3", "password3", "us", 900f, 1, 8));
        userDao.insert(new UserInst("user4", "password4", "us", 2000f, 50, 20));
        userDao.insert(new UserInst("user5", "password5", "us", 1500f, 40, 35));
    }

    @Test
    public void testInsertThenGetChallenges() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert(1L, 2L);
        challengeDao.insert(3L, 2L);
        challengeDao.insert(4L, 5L);

        List<ChallengeEntity> challenges = challengeDao.getByParticipant(null, 2L);

        // then
        List<ChallengeEntity> expected = List.of(
            new ChallengeEntity(
                3L, "user3", 900f,
                2L, "user2", 1005f,
                null),
            new ChallengeEntity(
                1L, "user1", 1000f,
                2L, "user2", 1005f,
                null));
        Assertions.assertEquals(expected, challenges);
    }

    @Test
    public void testExpiration() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert(2L, 5L, new Timestamp(System.currentTimeMillis() - 2000));
        challengeDao.insert(2L, 4L, new Timestamp(System.currentTimeMillis() - 1000));
        challengeDao.insert(2L, 3L, new Timestamp(System.currentTimeMillis()));
        challengeDao.insert(2L, 1L, new Timestamp(System.currentTimeMillis()));

        challengeDao.deleteExpired(2L, Duration.ofMillis(500));

        List<ChallengeEntity> challenges = challengeDao.getByParticipant(2L, null);

        // then
        List<ChallengeEntity> expected = List.of(
            new ChallengeEntity(
                2L, "user2", 1005f,
                1L, "user1", 1000f,
                null),
            new ChallengeEntity(
                2L, "user2", 1005f,
                3L, "user3", 900f,
                null));
        Assertions.assertEquals(expected, challenges);
    }

    @Test
    public void testDelete() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert(1L, 2L);
        int count = challengeDao.delete(1L, 2L);
        List<ChallengeEntity> challenges = challengeDao.getByParticipant(1L, null);

        // then
        Assertions.assertEquals(List.of(), challenges);
        Assertions.assertEquals(1, count);
    }
}
