package model;

import models.RankedUser;
import models.User;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

public class RankedUserTest {
    @Test
    public void testJoinRanks() {
        var entityList = new ArrayList<>(List.of(
            new User("id1", "user1", "us", 1000f, 0),
            new User("id3", "user3", "us", 1500f, 0),
            new User("id2", "user2", "us", 1250f, 0)));
        var rankedList = List.of(
            new RankedUser("id1", 3),
            new RankedUser("id3", 1),
            new RankedUser("id2", 2));

        RankedUser.joinRanks(rankedList, entityList);

        var expectedEntityList = List.of(
            new User("id3", "user3", "us", 1500f, 1),
            new User("id2", "user2", "us", 1250f, 2),
            new User("id1", "user1", "us", 1000f, 3));

        Assertions.assertEquals(expectedEntityList, entityList);
    }
}
