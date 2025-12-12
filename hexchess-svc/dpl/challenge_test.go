package dpl

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestChallengeExpiration(t *testing.T) {
	postgres, closer := BeforePgTxnTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-expiration")

	challenges, err := GetChallengesByParticipantOn(ctx, postgres.Query, ChallengeKey{int64(5), -1}, time.Unix(10000, 0))
	assert.NoError(t, err)

	assert.NoError(t, DeleteExpiredChallengesOn(ctx, postgres.Query, 5, time.Unix(10000, 0)))

	// get all challenges to prove that the deletion method worked.
	challengesDel, err := GetChallengesByParticipantOn(ctx, postgres.Query, ChallengeKey{int64(5), -1}, time.Unix(0, 0))
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
			TimeControl:       TcUnlimited,
			StartColor:        CsRandom,
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
			TimeControl:       TcUnlimited,
			StartColor:        CsRandom,
			MadeOn:            time.Time{},
		},
	}
	assert.Equal(t, expected, challenges)
	assert.Equal(t, expected, challengesDel)
}

func TestChallengeDeletion(t *testing.T) {
	pdb, closer := BeforePgTxnTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-delete")

	testUser := TestUserEntities[1] // ID: 2 will have no challenges at this point. if this changes, the test may break.

	_, err := InsertChallengeRet(ctx, pdb.Query, ChallengeInst{testUser.ID, 3, TcUnlimited, CsRandom, time.Time{}})
	assert.NoError(t, err)

	result, err := DeleteChallenge(ctx, pdb.Query, ChallengeKey{testUser.ID, 3})
	assert.NoError(t, err)

	challenges, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, ExpireChallengeThreshold)
	assert.NoError(t, err)

	assert.Empty(t, challenges)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 3, TimeControl: TcUnlimited, FirstColor: CsRandom}, result)
}
