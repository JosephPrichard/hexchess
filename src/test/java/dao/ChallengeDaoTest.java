package dao;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.Challenge;
import org.jetbrains.annotations.NotNull;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.sql.Timestamp;
import java.time.Duration;
import java.util.List;

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
        userDao.insert(new UserDao.UserInst("id1", "user1", "password1", "us", 1000f, 0, 0));
        userDao.insert(new UserDao.UserInst("id2", "user2", "password2", "us", 1005f, 1, 0));
        userDao.insert(new UserDao.UserInst("id3", "user3", "password3", "us", 900f, 1, 8));
        userDao.insert(new UserDao.UserInst("id4", "user4", "password4", "us", 2000f, 50, 20));
        userDao.insert(new UserDao.UserInst("id5", "user5", "password5", "us", 1500f, 40, 35));
    }

    @Test
    public void testGetChallenges() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert("id2", "id1");
        challengeDao.insert("id2", "id3");
        challengeDao.insert("id5", "id4");

        List<Challenge> challenges = challengeDao.getByParticipant(null, "id2");

        // then
        List<Challenge> expected = List.of(
            new Challenge(
                "id2", "user2", 1005f,
                "id1", "user1", 1000f,
                Challenge.Status.PENDING, null),
            new Challenge(
                "id2", "user2", 1005f,
                "id3", "user3", 900f,
                Challenge.Status.PENDING, null));
        Assertions.assertEquals(expected, challenges);
    }

    @Test
    public void testExpiration() throws InterruptedException {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert("id2", "id5", new Timestamp(System.currentTimeMillis() - 2000));
        challengeDao.insert("id2", "id4", new Timestamp(System.currentTimeMillis() - 1000));
        challengeDao.insert("id2", "id3", new Timestamp(System.currentTimeMillis()));
        challengeDao.insert("id2", "id1", new Timestamp(System.currentTimeMillis()));

        challengeDao.deleteExpired("id2", Duration.ofMillis(500));

        List<Challenge> challenges = challengeDao.getByParticipant(null, "id2");

        // then
        List<Challenge> expected = List.of(
            new Challenge(
                "id2", "user2", 1005f,
                "id3", "user3", 900f,
                Challenge.Status.PENDING, null),
            new Challenge(
                "id2", "user2", 1005f,
                "id1", "user1", 1000f,
                Challenge.Status.PENDING, null));
        Assertions.assertEquals(expected, challenges);
    }
}
