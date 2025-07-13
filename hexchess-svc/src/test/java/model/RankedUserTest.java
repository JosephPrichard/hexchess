package model;

import models.entities.UserRankEntity;
import models.entities.UserEntity;
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
        List<UserRankEntity> rankedList = List.of(
            new UserRankEntity(1L, 3),
            new UserRankEntity(3L, 1),
            new UserRankEntity(2L, 2));

        UserRankEntity.joinRanks(rankedList, entityList);

        List<UserEntity> expectedEntityList = List.of(
            new UserEntity(3L, "user3", "us", 1500f, 1),
            new UserEntity(2L, "user2", "us", 1250f, 2),
            new UserEntity(1L, "user1", "us", 1000f, 3));

        Assertions.assertEquals(expectedEntityList, entityList);
    }
}
