package dao;

import io.zonky.test.db.postgres.embedded.EmbeddedPostgres;
import models.User;
import org.junit.jupiter.api.*;
import utils.Config;

import javax.sql.DataSource;
import java.io.IOException;
import java.util.List;

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
        userDao.insert(new UserDao.UserInst("id1", "user1", "password1", "us", 1000f, 0, 0));
        userDao.insert(new UserDao.UserInst("id2", "user2", "password2", "us", 1005f, 1, 0));
        userDao.insert(new UserDao.UserInst("id3", "user3", "password3", "us", 900f, 1, 8));
        userDao.insert(new UserDao.UserInst("id4", "user4", "password4", "us", 2000f, 50, 20));
        userDao.insert(new UserDao.UserInst("id5", "user5", "password5", "us", 1500f, 40, 35));
    }

    @Test
    public void testInsertThenVerify() {
        // when
        var inst1 = userDao.insert("user1", "password1");
        var inst2 = userDao.insert("user2", "password2");
        var inst3 = userDao.insert("user3", "password3");

        var player1 = userDao.verify("user1", "password1");
        var player2 = userDao.verify("user2", "password2");
        var player3 = userDao.verify("user2", "wrong-password");
        var player4 = userDao.verify("user3", "password3");
        var player5 = userDao.verify("user1", "password3");

        // then
        Assertions.assertEquals(player1.getId(), inst1.getNewId());
        Assertions.assertEquals(player2.getId(), inst2.getNewId());
        Assertions.assertNull(player3);
        Assertions.assertEquals(player4.getId(), inst3.getNewId());
        Assertions.assertNull(player5);
    }

    @Test
    public void testUpdateStats() {
        // given
        createTestData(userDao);

        // when
        var changeSet = userDao.updateStats("id1", "id2");
        var actualUsers1 = userDao.getById("id1");
        var actualUsers2 = userDao.getById("id2");

        changeSet.roundElo();
        actualUsers1.roundElo();
        actualUsers2.roundElo();

        // then
        var expectedUsers1 = new User("id1", "user1", "us", 1015f, 1015f, 1, 0, 0, "", null);
        var expectedUsers2 = new User("id2", "user2", "us", 990f, 1005f, 1, 1, 0, "", null);

        Assertions.assertEquals(new UserDao.EloChangeSet(1015f, 990f), changeSet);
        Assertions.assertEquals(expectedUsers1, actualUsers1);
        Assertions.assertEquals(expectedUsers2, actualUsers2);
    }

    @Test
    public void testUpdateUser() {
        // given
        createTestData(userDao);

        // when
        userDao.updateUser("id1", "user1-changed", null, "Testing123");
        userDao.updateUser("id2", "user2-changed", "eu", null);

        var actualUser1 = userDao.getById("id1");
        var actualUser2 = userDao.getById("id2");

        // then
        var expectedUser1 = new User("id1", "user1-changed", "us", 1000f, 1000f, 0, 0, 0, "Testing123", null);
        var expectedUser2 = new User("id2", "user2-changed", "eu", 1005f, 1005f, 1, 0, 0, "", null);

        Assertions.assertEquals(expectedUser1, actualUser1);
        Assertions.assertEquals(expectedUser2, actualUser2);
    }

    @Test
    public void testUpdatePassword() {
        // given
        createTestData(userDao);

        // when
        userDao.updatePassword("id1", "password-new");

        var user = userDao.getById("id1");
        var player = userDao.verify("user1", "password-new");

        // then
        Assertions.assertEquals(player.getId(), user.getId());
    }

    @Test
    public void testGetLeaderboard() {
        // given
        createTestData(userDao);

        // when
        var actualUserList = userDao.getLeaderboard(1, 5);

        // then
        var expectedUserList = List.of(
            new User("id4", "user4", "us", 2000f, 0f, 50, 20, 1, null, null),
            new User("id5", "user5", "us", 1500f, 0f, 40, 35, 2, null, null),
            new User("id2", "user2", "us", 1005f, 0f, 1, 0, 3, null, null),
            new User("id1", "user1", "us", 1000f, 0f, 0, 0, 4, null, null),
            new User("id3", "user3", "us", 900f, 0f, 1, 8, 5, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }

    @Test
    public void testGetByIds() {
        // given
        createTestData(userDao);

        // when
        var actualUserList = userDao.getByIds(List.of("id1", "id2"));

        // then
        var expectedUserList = List.of(
            new User("id1", "user1", "us", 1000f, 0f, 0, 0, 0, null, null),
            new User("id2", "user2", "us", 1005f, 0f, 1, 0, 0, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }

    @Test
    public void testByIdWithRank() {
        // given
        createTestData(userDao);

        // when
        var actualUser = userDao.getByIdWithRank("id1");

        // then
        var expectedUser = new User("id1", "user1", "us", 1000f, 1000f, 0, 0, 4, "", null);
        Assertions.assertEquals(expectedUser, actualUser);
    }

    @Test
    public void searchByName() {
        // given
        createTestData(userDao);
        userDao.insert(new UserDao.UserInst("id6", "johnny", "password6", "us", 0f, 0, 0));
        userDao.insert(new UserDao.UserInst("id7", "john", "password7", "us", 0f, 0, 0));

        // when
        var actualUserList = userDao.searchByName("john", 1, 20);

        // then
        var expectedUserList = List.of(
            new User("id6", "johnny", "us", 0f, 0f, 0, 0, 1, null, null),
            new User("id7", "john", "us", 0f, 0f, 0, 0, 2, null, null));
        Assertions.assertEquals(expectedUserList, actualUserList);
    }

    @Test
    public void testCountUsers() {
        // given
        createTestData(userDao);

        // when
        var count = userDao.countUsers();

        // then
        Assertions.assertEquals(5, count);
    }
}
