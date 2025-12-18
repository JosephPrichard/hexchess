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

	ctx := context.WithValue(t.Context(), util.Trace, "testing-expiration")

	// when
	challenges, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(10000, 0))
	assert.NoError(t, err)

	assert.NoError(t, DeleteExpiredChallengesOn(ctx, pdb.Query, 5, time.Unix(10000, 0)))

	// get all challenges to prove that the deletion worked
	challengesDel, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(0, 0))
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
			MadeOn:            time.Unix(20500, 0),
			ExpiresOn:         time.Unix(20500, 0).Add(ExpireChallengeThreshold),
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
			MadeOn:            time.Unix(19500, 0),
			ExpiresOn:         time.Unix(19500, 0).Add(ExpireChallengeThreshold),
		},
	}
	assert.Equal(t, expected, challenges)
	assert.Equal(t, expected, challengesDel)
}

func TestChallengeInsertAndDelete(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-delete")

	testUser := TestUserEntities[1] // user ID: 2 will have no challenges at this point

	timeOn := TestTimeNow

	// when
	err := InsertChallenge(ctx, pdb.Query, ChallengeInst{
		ChallengerID: testUser.ID,
		ChallengeeID: 3,
		TimeControl:  TcUnlimited,
		StartColor:   ColorRandom,
		MadeOn:       timeOn,
	})
	assert.NoError(t, err)

	challengesBeforeDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, timeOn)
	assert.NoError(t, err)

	dr, err := DeleteChallenge(ctx, pdb.Query, ChallengeKey{testUser.ID, 3})
	assert.NoError(t, err)

	challengesAfterDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, timeOn)
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
		MadeOn:            timeOn.Local(),
		ExpiresOn:         timeOn.Local().Add(ExpireChallengeThreshold),
	}}
	assert.Equal(t, expChallengesBefore, challengesBeforeDelete)
	assert.Empty(t, challengesAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 3, TimeControl: TcUnlimited, FirstColor: ColorRandom}, dr)
}
