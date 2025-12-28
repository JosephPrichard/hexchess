package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

func TestChallengeExpiration(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-expiration")

	// when
	challenges, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(10000, 0))
	require.NoError(t, err)

	require.NoError(t, DeleteExpiredChallengesOn(ctx, pdb.Query, 5, time.Unix(10000, 0)))

	// get all challenges to prove that the deletion worked
	challengesDel, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{int64(5), -1}, time.Unix(0, 0))
	require.NoError(t, err)

	// then
	expected := []ChallengeEntity{TestChallengeEntities[2], TestChallengeEntities[3]}
	assert.Equal(t, expected, challenges)
	assert.Equal(t, expected, challengesDel)
}

func TestInsertChallenge(t *testing.T) {
	for _, test := range []struct {
		name         string
		challengerID int64
		challengeeID int64
		wantErr      error
	}{
		{
			name:         "cannot challenge self",
			challengerID: 1,
			challengeeID: 1,
			wantErr:      ErrSelfChallenge,
		},
		{
			name:         "invalid user",
			challengerID: 9000,
			challengeeID: 1,
			wantErr:      ErrParticipantConflict,
		},
		{
			name:         "duplicate challenge",
			challengerID: 1,
			challengeeID: 2,
			wantErr:      ErrDuplicateChallenge,
		},
		{
			name:         "valid challenge",
			challengerID: 2,
			challengeeID: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// given
			pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
			defer closer()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			// when
			err := InsertChallenge(ctx, pdb.Query, ChallengeInst{
				ChallengerID: test.challengerID,
				ChallengeeID: test.challengeeID,
				Mode:         ModeCorrespondence7,
				StartColor:   Random,
			})

			// then
			assert.Equal(t, test.wantErr, err)
		})
	}
}

func TestChallengeEchoDelete(t *testing.T) {
	// given
	pdb, closer := db.BeforePostgresTest(t, true, InsertTestData)
	defer closer()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-delete")

	testUser := TestUserEntities[1] // user ID: 2 will have no challenges at this point

	timeOn := TestTimeNow

	// when
	err := InsertChallenge(ctx, pdb.Query, ChallengeInst{
		ChallengerID: testUser.ID,
		ChallengeeID: 3,
		Mode:         ModeCorrespondence7,
		StartColor:   Random,
		MadeOn:       timeOn,
	})
	require.NoError(t, err)

	challengesBeforeDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, timeOn)
	require.NoError(t, err)

	dr, err := DeleteChallenge(ctx, pdb.Query, ChallengeKey{testUser.ID, 3})
	require.NoError(t, err)

	challengesAfterDelete, err := GetChallengesByParticipant(ctx, pdb.Query, ChallengeKey{testUser.ID, -1}, timeOn)
	require.NoError(t, err)

	// then
	wantChallengesBefore := []ChallengeEntity{{
		ChallengerID:      2,
		ChallengerName:    "user2",
		ChallengerCountry: "us",
		ChallengerElo:     1000,
		ChallengeeID:      3,
		ChallengeeName:    "user3",
		ChallengeeCountry: "us",
		ChallengeeElo:     900,
		Mode:              ModeCorrespondence7.String(),
		StartColor:        Random.String(),
		MadeOn:            timeOn.Local(),
		ExpiresOn:         timeOn.Local().Add(ExpireChallengeThreshold),
	}}
	assert.Equal(t, wantChallengesBefore, challengesBeforeDelete)
	assert.Empty(t, challengesAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: testUser.ID, ChallengeeID: 3, Mode: ModeCorrespondence7, FirstColor: Random}, dr)
}
