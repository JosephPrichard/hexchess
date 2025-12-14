package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"
)

func TestChallengeExpiration(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-expiration")

	// when
	challenges, err := GetChallengesByParticipantOn(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(10000, 0))
	assert.NoError(t, err)

	assert.NoError(t, DeleteExpiredChallengesOn(ctx, pdb.Query, 5, time.Unix(10000, 0)))

	// get all challenges to prove that the deletion worked
	challengesDel, err := GetChallengesByParticipantOn(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(0, 0))
	assert.NoError(t, err)

	// then
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
			StartColor:        ColorRandom,
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
			StartColor:        ColorRandom,
		},
	}
	util.AssertEqualIgnoring(t, expected, challenges, ChallengeEntityCmpOpts)
	util.AssertEqualIgnoring(t, expected, challengesDel, ChallengeEntityCmpOpts)
}

func TestChallengeInsertAndDelete(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-delete")

	testUser := TestUserEntities[1] // user ID: 2 will have no challenges at this point

	// when
	err := InsertChallenge(ctx, pdb.Query, ChallengeInst{testUser.ID, 3, TcUnlimited, ColorRandom, time.Time{}})
	assert.NoError(t, err)

	challengesBeforeDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, ExpireChallengeThreshold)
	assert.NoError(t, err)

	dr, err := DeleteChallenge(ctx, pdb.Query, ChallengeKey{testUser.ID, 3})
	assert.NoError(t, err)

	challengesAfterDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, ExpireChallengeThreshold)
	assert.NoError(t, err)

	// then
	expChallengesBefore := []ChallengeEntity{{
		ChallengerID:      2,
		ChallengerName:    "user2",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeID:      3,
		ChallengeeName:    "user3",
		ChallengeeCountry: "us",
		ChallengeeElo:     900,
		TimeControl:       TcUnlimited,
		StartColor:        ColorRandom,
	}}
	util.AssertEqualIgnoring(t, expChallengesBefore, challengesBeforeDelete, ChallengeEntityCmpOpts)
	assert.Empty(t, challengesAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 3, TimeControl: TcUnlimited, FirstColor: ColorRandom}, dr)
}
