package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/logs"
	"testing"
	"time"
)

func TestGetChallenges(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-insert-get-challenges")

	testUser := createTestUser(t, pgDB.Q, UserInst{Username: "user1", Password: "password1", Country: "us", Elo: 1000})

	c1, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{1, testUser.ID, Unlimited, Random, time.Time{}})
	assert.NoError(t, err)
	c2, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{3, testUser.ID, Unlimited, Random, time.Time{}})
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, NoChallengeID, testUser.ID, ExpireChallengeThreshold)
	assert.NoError(t, err)

	c1.MadeOn = time.Time{}
	c2.MadeOn = time.Time{}
	for i := range challenges {
		challenges[i].MadeOn = time.Time{}
	}

	challenge1 := ChallengeEntity{
		ChallengerID:      1,
		ChallengerName:    "user1",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeId:      testUser.ID,
		ChallengeeName:    testUser.Username,
		ChallengeeCountry: testUser.Country,
		ChallengeeElo:     testUser.Elo,
		TimeControl:       Unlimited,
		StartColor:        Random,
		MadeOn:            time.Time{},
	}

	challenge2 := ChallengeEntity{
		ChallengerID:      3,
		ChallengerName:    "user3",
		ChallengerCountry: "us",
		ChallengerElo:     900,
		ChallengeeId:      testUser.ID,
		ChallengeeName:    testUser.Username,
		ChallengeeCountry: testUser.Country,
		ChallengeeElo:     testUser.Elo,
		TimeControl:       Unlimited,
		StartColor:        Random,
		MadeOn:            time.Time{},
	}

	expectedList := []ChallengeEntity{challenge2, challenge1}
	assert.Equal(t, expectedList, challenges)
	assert.Equal(t, challenge1, c1)
	assert.Equal(t, challenge2, c2)
}

func TestChallengeExpiration(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-expiration")
	now := time.Now()

	testUser := createTestUser(t, pgDB.Q, UserInst{Username: "user1", Password: "password1", Country: "us", Elo: 1000})

	insts := []ChallengeInst{
		{testUser.ID, 2, Unlimited, Random, now.Add(-2 * time.Second)},
		{testUser.ID, 4, Unlimited, Random, now.Add(-1 * time.Second)},
		{testUser.ID, 3, Unlimited, Random, now},
		{testUser.ID, 1, Unlimited, Random, now},
	}
	for _, c := range insts {
		assert.NoError(t, InsertChallenge(ctx, pgDB.Q, c))
	}

	assert.NoError(t, DeleteExpiredChallenges(ctx, pgDB.Q, testUser.ID, 500*time.Millisecond))

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, testUser.ID, NoChallengeID, ExpireChallengeThreshold)
	assert.NoError(t, err)

	for i := range challenges {
		challenges[i].MadeOn = time.Time{}
	}
	expected := []ChallengeEntity{
		{
			ChallengerID:      testUser.ID,
			ChallengerName:    testUser.Username,
			ChallengerCountry: testUser.Country,
			ChallengerElo:     testUser.Elo,
			ChallengeeId:      1,
			ChallengeeName:    "user1",
			ChallengeeCountry: "us",
			ChallengeeElo:     1000,
			TimeControl:       Unlimited,
			StartColor:        Random,
			MadeOn:            time.Time{},
		},
		{
			ChallengerID:      testUser.ID,
			ChallengerName:    testUser.Username,
			ChallengerCountry: testUser.Country,
			ChallengerElo:     testUser.Elo,
			ChallengeeId:      3,
			ChallengeeName:    "user3",
			ChallengeeCountry: "us",
			ChallengeeElo:     900,
			TimeControl:       Unlimited,
			StartColor:        Random,
			MadeOn:            time.Time{},
		},
	}
	assert.Equal(t, expected, challenges)
}

func TestChallengeDeletion(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), logs.TraceKey, "testing-delete")

	testUser := createTestUser(t, pgDB.Q, UserInst{Username: "user1", Password: "password1", Country: "us", Elo: 1000})

	_, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{testUser.ID, 2, Unlimited, Random, time.Time{}})
	assert.NoError(t, err)

	result, err := DeleteChallenge(ctx, pgDB.Q, testUser.ID, 2)
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, testUser.ID, NoChallengeID, ExpireChallengeThreshold)
	assert.NoError(t, err)

	assert.Empty(t, challenges)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 2, TimeControl: Unlimited, FirstColor: Random}, result)
}
