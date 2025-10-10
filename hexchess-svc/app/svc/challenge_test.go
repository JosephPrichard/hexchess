package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	Challenge1 = ChallengeEntity{
		ChallengerId:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeId:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		TimeControl:       "UNLIMITED",
		StartColor:        "RANDOM",
		MadeOn:            time.Time{},
	}

	Challenge2 = ChallengeEntity{
		ChallengerId:      3,
		ChallengerName:    "user3",
		ChallengerCountry: "us",
		ChallengerElo:     900,
		ChallengeeId:      2,
		ChallengeeName:    "user2",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		TimeControl:       "UNLIMITED",
		StartColor:        "RANDOM",
		MadeOn:            time.Time{},
	}

	ExpiredChallenge1 = ChallengeEntity{
		ChallengerId:      2,
		ChallengerName:    "user2",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeId:      1,
		ChallengeeName:    "user1",
		ChallengeeCountry: "us",
		ChallengeeElo:     1000,
		TimeControl:       "UNLIMITED",
		StartColor:        "RANDOM",
		MadeOn:            time.Time{},
	}

	ExpiredChallenge2 = ChallengeEntity{
		ChallengerId:      2,
		ChallengerName:    "user2",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeId:      3,
		ChallengeeName:    "user3",
		ChallengeeCountry: "us",
		ChallengeeElo:     900,
		TimeControl:       "UNLIMITED",
		StartColor:        "RANDOM",
		MadeOn:            time.Time{},
	}
)

func TestInsertThenGetChallenges(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-insert-get-challenges")
	createTestUsers(t, pgDB.Q)

	c1, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{1, 2, "UNLIMITED", "RANDOM", time.Time{}})
	assert.NoError(t, err)
	c2, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{3, 2, "UNLIMITED", "RANDOM", time.Time{}})
	assert.NoError(t, err)
	_, err = InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{4, 5, "UNLIMITED", "RANDOM", time.Time{}})
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, nil, pointerOf(int64(2)), ExpireChallengeThreshold)
	assert.NoError(t, err)

	// clear fields that we don't want to assert
	c1.MadeOn = time.Time{}
	c2.MadeOn = time.Time{}
	for i := range challenges {
		challenges[i].MadeOn = time.Time{}
	}

	expectedList := []ChallengeEntity{Challenge2, Challenge1}
	assert.Equal(t, expectedList, challenges)
	assert.Equal(t, Challenge1, c1)
	assert.Equal(t, Challenge2, c2)
}

func TestExpiration(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-expiration")
	createTestUsers(t, pgDB.Q)

	now := time.Now()
	assert.NoError(t, InsertChallenge(ctx, pgDB.Q, ChallengeInst{2, 5, "UNLIMITED", "RANDOM", now.Add(-2 * time.Second)}))
	assert.NoError(t, InsertChallenge(ctx, pgDB.Q, ChallengeInst{2, 4, "UNLIMITED", "RANDOM", now.Add(-1 * time.Second)}))
	assert.NoError(t, InsertChallenge(ctx, pgDB.Q, ChallengeInst{2, 3, "UNLIMITED", "RANDOM", now}))
	assert.NoError(t, InsertChallenge(ctx, pgDB.Q, ChallengeInst{2, 1, "UNLIMITED", "RANDOM", now}))

	assert.NoError(t, DeleteExpiredChallenges(ctx, pgDB.Q, 2, 500*time.Millisecond))

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, pointerOf(int64(2)), nil, ExpireChallengeThreshold)
	assert.NoError(t, err)

	// clear fields that we don't want to assert
	for i := range challenges {
		challenges[i].MadeOn = time.Time{}
	}

	expected := []ChallengeEntity{ExpiredChallenge1, ExpiredChallenge2}
	assert.Equal(t, expected, challenges)
}

func TestDeleteChallenge(t *testing.T) {
	pgDB, closer := beforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), TraceKey, "test-delete")
	createTestUsers(t, pgDB.Q)

	_, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{1, 2, "UNLIMITED", "RANDOM", time.Time{}})
	assert.NoError(t, err)

	result, err := DeleteChallenge(ctx, pgDB.Q, 1, 2)
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, pointerOf(int64(1)), nil, ExpireChallengeThreshold)
	assert.NoError(t, err)

	assert.Empty(t, challenges)
	assert.Equal(t, DeleteResult{ChallengerID: 1, ChallengeeID: 2, TimeControl: "UNLIMITED", StartColor: "RANDOM"}, result)
}
