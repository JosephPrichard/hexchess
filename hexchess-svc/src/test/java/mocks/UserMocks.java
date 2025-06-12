package mocks;

import models.entities.UserEntity;
import models.views.UserView;

public class UserMocks {
    public static final UserEntity USER_ENTITY1 = UserEntity.builder()
        .id(1L)
        .username("user1")
        .country("us")
        .elo(1000f)
        .highestElo(1200f)
        .wins(20)
        .losses(20)
        .rank(0)
        .bio("Biography 1")
        .joinedOn(null)
        .build();
    public static final UserEntity USER_ENTITY2 = UserEntity.builder()
        .id(2L)
        .username("user2")
        .country("us")
        .elo(800f)
        .highestElo(1000f)
        .wins(80)
        .losses(160)
        .rank(0)
        .bio("Biography 2")
        .joinedOn(null)
        .build();

    public static final UserView USER_VIEW1 = UserView.builder()
        .id(1L)
        .username("user1")
        .country("us")
        .elo(1000)
        .highestElo(1200)
        .wins(20)
        .losses(20)
        .rank(0)
        .bio("Biography 1")
        .joinedOn("")
        .total(40)
        .winRate(50)
        .rank(1)
        .build();
    public static final UserView USER_VIEW2 = UserView.builder()
        .id(2L)
        .username("user2")
        .country("us")
        .elo(800)
        .highestElo(1000)
        .wins(80)
        .losses(160)
        .rank(0)
        .bio("Biography 2")
        .joinedOn("")
        .total(240)
        .winRate(33)
        .rank(2)
        .build();
}
