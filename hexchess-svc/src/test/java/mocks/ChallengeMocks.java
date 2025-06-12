package mocks;

import models.entities.ChallengeEntity;

public class ChallengeMocks {
    public static final ChallengeEntity CHALLENGE1 = ChallengeEntity.builder()
        .challengerId(1L)
        .challengerName("user1")
        .challengerCountry("us")
        .challengerElo(1000f)
        .challengeeId(2L)
        .challengeeName("user2")
        .challengeeCountry("us")
        .challengeeElo(1005f)
        .timeControl("UNLIMITED")
        .startColor("RANDOM")
        .madeOn(null)
        .build();

    public static final ChallengeEntity CHALLENGE2 = ChallengeEntity.builder()
        .challengerId(3L)
        .challengerName("user3")
        .challengerCountry("us")
        .challengerElo(900f)
        .challengeeId(2L)
        .challengeeName("user2")
        .challengeeCountry("us")
        .challengeeElo(1005f)
        .timeControl("UNLIMITED")
        .startColor("RANDOM")
        .madeOn(null)
        .build();

    public static final ChallengeEntity EXPIRED_CHALLENGE1 = ChallengeEntity.builder()
        .challengerId(2L)
        .challengerName("user2")
        .challengerCountry("us")
        .challengerElo(1005f)
        .challengeeId(1L)
        .challengeeName("user1")
        .challengeeCountry("us")
        .challengeeElo(1000f)
        .timeControl("UNLIMITED")
        .startColor("RANDOM")
        .madeOn(null)
        .build();

    public static final ChallengeEntity EXPIRED_CHALLENGE2 = ChallengeEntity.builder()
        .challengerId(2L)
        .challengerName("user2")
        .challengerCountry("us")
        .challengerElo(1005f)
        .challengeeId(3L)
        .challengeeName("user3")
        .challengeeCountry("us")
        .challengeeElo(900f)
        .timeControl("UNLIMITED")
        .startColor("RANDOM")
        .madeOn(null)
        .build();
}
