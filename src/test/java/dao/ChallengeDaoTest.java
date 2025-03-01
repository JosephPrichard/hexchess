package dao;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.Challenge;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.time.Duration;
import java.util.List;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class ChallengeDaoTest {

    public EmbeddedPostgres pg;
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
    public void testInsertThenUpdateThenSelect() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insertStatus("id2", "id1");
        challengeDao.insertStatus("id2", "id3");
        challengeDao.insertStatus("id5", "id4");
        challengeDao.updateStatus("id2", "id3", Challenge.ACCEPTED);

        var challenges = challengeDao.getByChallengee("id2");

        // then
        var expected = List.of(
            new Challenge(
                "id2", "user2", "us", 1005f,
                "id1", "user1", "us", 1000f,
                Challenge.PENDING, null),
            new Challenge(
                "id2", "user2", "us", 1005f,
                "id3", "user3", "us", 900f,
                Challenge.ACCEPTED, null));
        Assertions.assertEquals(expected, challenges);
    }

    @Test
    public void testDeleteThenSelect() throws InterruptedException {
        // given
        createTestData(userDao);

        // when
        challengeDao.insertStatus("id2", "id1");
        Thread.sleep(10);
        challengeDao.insertStatus("id2", "id3");

        challengeDao.deleteExpired("id2", Duration.ofMillis(5));

        var challenges = challengeDao.getByChallengee("id2");

        // then
        var expected = List.of(
            new Challenge(
                "id2", "user2", "us", 1005f,
                "id3", "user3", "us", 900f,
                Challenge.PENDING, null));
        Assertions.assertEquals(expected, challenges);
    }
}
