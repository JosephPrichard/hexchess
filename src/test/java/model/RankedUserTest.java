package model;

import models.RankedUser;
import models.UserEntity;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

public class RankedUserTest {
    @Test
    public void testJoinRanks() {
        List<UserEntity> entityList = new ArrayList<>(List.of(
            new UserEntity(1L, "user1", "us", 1000f, 0),
            new UserEntity(3L, "user3", "us", 1500f, 0),
            new UserEntity(2L, "user2", "us", 1250f, 0)));
        List<RankedUser> rankedList = List.of(
            new RankedUser(1L, 3),
            new RankedUser(3L, 1),
            new RankedUser(2L, 2));

        RankedUser.joinRanks(rankedList, entityList);

        List<UserEntity> expectedEntityList = List.of(
            new UserEntity(3L, "user3", "us", 1500f, 1),
            new UserEntity(2L, "user2", "us", 1250f, 2),
            new UserEntity(1L, "user1", "us", 1000f, 3));

        Assertions.assertEquals(expectedEntityList, entityList);
    }
}
