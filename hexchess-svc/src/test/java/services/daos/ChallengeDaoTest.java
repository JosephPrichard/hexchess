package services.daos;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.entities.ChallengeEntity;
import org.junit.jupiter.api.*;
import services.daos.ChallengeDao;
import services.daos.UserDao;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.sql.Timestamp;
import java.time.Duration;
import java.util.List;

import static services.daos.UserDao.*;

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
        ChallengeEntity entity1 = challengeDao.insert(1L, 2L, "UNLIMITED", "RANDOM");
        ChallengeEntity entity2 = challengeDao.insert(3L, 2L, "UNLIMITED", "RANDOM");
        challengeDao.insert(4L, 5L, "UNLIMITED", "RANDOM");

        List<ChallengeEntity> entityList = challengeDao.getByParticipant(null, 2L);

        // then
        ChallengeEntity expectedEntity1 = new ChallengeEntity(
            1L, "user1", "us", 1000f,
            2L, "user2", "us", 1005f,
            "UNLIMITED", "RANDOM", null);
        ChallengeEntity expectedEntity2 = new ChallengeEntity(
            3L, "user3", "us", 900f,
            2L, "user2", "us", 1005f,
            "UNLIMITED", "RANDOM", null);
        List<ChallengeEntity> expectedList = List.of(expectedEntity2, expectedEntity1);
        Assertions.assertEquals(expectedList, entityList);
        Assertions.assertEquals(expectedEntity1, entity1);
        Assertions.assertEquals(expectedEntity2, entity2);
    }

    @Test
    public void testExpiration() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert(2L, 5L, "UNLIMITED", "RANDOM", new Timestamp(System.currentTimeMillis() - 2000));
        challengeDao.insert(2L, 4L, "UNLIMITED", "RANDOM", new Timestamp(System.currentTimeMillis() - 1000));
        challengeDao.insert(2L, 3L, "UNLIMITED", "RANDOM", new Timestamp(System.currentTimeMillis()));
        challengeDao.insert(2L, 1L, "UNLIMITED", "RANDOM", new Timestamp(System.currentTimeMillis()));

        challengeDao.deleteExpired(2L, Duration.ofMillis(500));

        List<ChallengeEntity> challenges = challengeDao.getByParticipant(2L, null);

        // then
        List<ChallengeEntity> expected = List.of(
            new ChallengeEntity(
                2L, "user2", "us", 1005f,
                1L, "user1", "us", 1000f,
                "UNLIMITED", "RANDOM", null),
            new ChallengeEntity(
                2L, "user2", "us", 1005f,
                3L, "user3", "us", 900f,
                "UNLIMITED", "RANDOM",     null));
        Assertions.assertEquals(expected, challenges);
    }

    @Test
    public void testDelete() {
        // given
        createTestData(userDao);

        // when
        challengeDao.insert(1L, 2L, "UNLIMITED", "RANDOM");
        ChallengeDao.DeleteResult result = challengeDao.delete(1L, 2L);
        List<ChallengeEntity> challenges = challengeDao.getByParticipant(1L, null);

        // then
        Assertions.assertEquals(List.of(), challenges);
        Assertions.assertEquals(new ChallengeDao.DeleteResult(1L, 2L, "UNLIMITED", "RANDOM"), result);
    }
}
