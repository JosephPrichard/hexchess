package daos;

import daos.UserDao;
import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.UserEntity;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.util.List;

import static daos.UserDao.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class UserDaoTest {

    private EmbeddedPostgres pg;
    private DataSource ds;
    private UserDao userDao;

    @BeforeAll
    public void beforeAll() throws IOException {
        pg = EmbeddedPostgres.builder().start();
        ds = pg.getPostgresDatabase();

        userDao = new UserDao(ds);
    }

    @BeforeEach
    public void beforeEach() {
        Config.createSchema(ds);
    }

    @AfterAll
    public void afterAll() throws IOException {
        pg.close();
    }

    public static void createTestData(UserDao userDao) {
        userDao.insert(new UserInst("user1", "password1", "us", 1000f, 0, 0));
        userDao.insert(new UserInst("user2", "password2", "us", 1005f, 1, 0));
        userDao.insert(new UserInst("user3", "password3", "us", 900f, 1, 8));
        userDao.insert(new UserInst("user4", "password4", "us", 2000f, 50, 20));
        userDao.insert(new UserInst("user5", "password5", "us", 1500f, 40, 35));
    }

    @Test
    public void testInsertThenVerify() {
        // when
        UserEntity user1 = userDao.insert("user1", "password1");
        UserEntity user2 = userDao.insert("user2", "password2");
        UserEntity user3 = userDao.insert("user3", "password3");

        VerifiedUser verified1 = userDao.verify("user1", "password1");
        VerifiedUser verified2 = userDao.verify("user2", "password2");
        VerifiedUser verified3 = userDao.verify("user2", "wrong-password");
        VerifiedUser verified4 = userDao.verify("user3", "password3");
        VerifiedUser verified5 = userDao.verify("user1", "password3");

        // then
        Assertions.assertEquals(verified1.getId(), user1.getId());
        Assertions.assertEquals(verified2.getId(), user2.getId());
        Assertions.assertNull(verified3);
        Assertions.assertEquals(verified4.getId(), user3.getId());
        Assertions.assertNull(verified5);
    }

    @Test
    public void testUpdateStats() {
        // given
        createTestData(userDao);

        // when
        EloChangeSet changeSet = userDao.updateStats(1L, 2L);
        UserEntity actualUsers1 = userDao.getById(1L);
        UserEntity actualUsers2 = userDao.getById(2L);

        changeSet.roundElo();
        actualUsers1.roundElo();
        actualUsers2.roundElo();

        // then
        UserEntity expectedUsers1 = new UserEntity(1L, "user1", "us", 1015f, 1015f, 1, 0, 0, "", null);
        UserEntity expectedUsers2 = new UserEntity(2L, "user2", "us", 990f, 1005f, 1, 1, 0, "", null);

        Assertions.assertEquals(new EloChangeSet(1015f, 990f), changeSet);
        Assertions.assertEquals(expectedUsers1, actualUsers1);
        Assertions.assertEquals(expectedUsers2, actualUsers2);
    }

    @Test
    public void testUpdateUser() {
        // given
        createTestData(userDao);

        // when
        userDao.updateUser(1L, "user1-changed", null, "Testing123");
        userDao.updateUser(2L, "user2-changed", "eu", null);

        UserEntity actualUser1 = userDao.getById(1L);
        UserEntity actualUser2 = userDao.getById(2L);

        // then
        UserEntity expectedUser1 = new UserEntity(1L, "user1-changed", "us", 1000f, 1000f, 0, 0, 0, "Testing123", null);
        UserEntity expectedUser2 = new UserEntity(2L, "user2-changed", "eu", 1005f, 1005f, 1, 0, 0, "", null);

        Assertions.assertEquals(expectedUser1, actualUser1);
        Assertions.assertEquals(expectedUser2, actualUser2);
    }

    @Test
    public void testUpdatePassword() {
        // given
        createTestData(userDao);

        // when
        userDao.updatePassword(1L, "password-new");

        UserEntity user = userDao.getById(1L);
        UserDao.VerifiedUser player = userDao.verify("user1", "password-new");

        // then
        Assertions.assertEquals(player.getId(), user.getId());
    }

    @Test
    public void testGetLeaderboard() {
        // given
        createTestData(userDao);

        // when
        List<UserEntity> actualUserList = userDao.getLeaderboard(1, 5);

        // then
        List<UserEntity> expectedUserList = List.of(
                new UserEntity(4L, "user4", "us", 2000f, 0f, 50, 20, 1, null, null),
                new UserEntity(5L, "user5", "us", 1500f, 0f, 40, 35, 2, null, null),
                new UserEntity(2L, "user2", "us", 1005f, 0f, 1, 0, 3, null, null),
                new UserEntity(1L, "user1", "us", 1000f, 0f, 0, 0, 4, null, null),
                new UserEntity(3L, "user3", "us", 900f, 0f, 1, 8, 5, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }

    @Test
    public void testGetByIds() {
        // given
        createTestData(userDao);

        // when
        List<UserEntity> actualUserList = userDao.getByIds(new Long[]{1L, 2L});

        // then
        List<UserEntity> expectedUserList = List.of(
                new UserEntity(1L, "user1", "us", 1000f, 0f, 0, 0, 0, null, null),
                new UserEntity(2L, "user2", "us", 1005f, 0f, 1, 0, 0, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }

    @Test
    public void searchByName() {
        // given
        createTestData(userDao);
        userDao.insert(new UserInst("johnny", "password6", "us", 0f, 0, 0));
        userDao.insert(new UserInst("john", "password7", "us", 0f, 0, 0));

        // when
        List<UserEntity> actualUserList = userDao.searchByName("john", 1, 20);

        // then
        List<UserEntity> expectedUserList = List.of(
                new UserEntity(6L, "johnny", "us", 0f, 0f, 0, 0, 1, null, null),
                new UserEntity(7L, "john", "us", 0f, 0f, 0, 0, 2, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }
}