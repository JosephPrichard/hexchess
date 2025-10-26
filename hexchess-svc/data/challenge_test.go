package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/lib"
	"testing"
	"time"
)

func TestChallengeExpiration(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-expiration")

	challenges, err := GetChallengesByParticipantOn(ctx, pgDB.Q, int64(5), -1, time.Unix(10000, 0))
	assert.NoError(t, err)

	assert.NoError(t, DeleteExpiredChallengesOn(ctx, pgDB.Q, 5, time.Unix(10000, 0)))

	// get all challenges to prove that the deletion method worked.
	challengesDel, err := GetChallengesByParticipantOn(ctx, pgDB.Q, int64(5), -1, time.Unix(0, 0))
	assert.NoError(t, err)

	for _, ca := range [][]ChallengeEntity{challenges, challengesDel} {
		for i := range ca {
			ca[i].MadeOn = time.Time{}
		}
	}

	expected := []ChallengeEntity{
		{
			ChallengerID:      5,
			ChallengerName:    "user5",
			ChallengerCountry: "us",
			ChallengerElo:     1500,
			ChallengeeID:      2,
			ChallengeeName:    "user2",
			ChallengeeCountry: "us",
			ChallengeeElo:     1000,
			TimeControl:       2,
			StartColor:        2,
			MadeOn:            time.Time{},
		},
		{
			ChallengerID:      5,
			ChallengerName:    "user5",
			ChallengerCountry: "us",
			ChallengerElo:     1500,
			ChallengeeID:      4,
			ChallengeeName:    "user4",
			ChallengeeCountry: "us",
			ChallengeeElo:     2000,
			TimeControl:       2,
			StartColor:        2,
			MadeOn:            time.Time{},
		},
	}
	assert.Equal(t, expected, challenges)
	assert.Equal(t, expected, challengesDel)
}

func TestChallengeDeletion(t *testing.T) {
	pgDB, closer := BeforeDbTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-delete")

	testUser := TestUserEntities[1] // ID: 2 will have no challenges at this point. if this changes, the test may break.

	_, err := InsertChallengeRet(ctx, pgDB.Q, ChallengeInst{testUser.ID, 3, Unlimited, Random, time.Time{}})
	assert.NoError(t, err)

	result, err := DeleteChallenge(ctx, pgDB.Q, testUser.ID, 3)
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pgDB.Q, testUser.ID, -1, ExpireChallengeThreshold)
	assert.NoError(t, err)

	assert.Empty(t, challenges)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 3, TimeControl: Unlimited, FirstColor: Random}, result)
}
