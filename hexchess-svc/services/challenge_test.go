package svc

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"

	"testing"
	"time"
)

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
			s := SetupStateTest(t, itest.UseTxn, itest.WithPostgres)
			defer s.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, test.name)

			// when
			err := s.InsertChallenge(ctx, ChallengeInst{
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

func TestGetChallengesByParticipant(t *testing.T) {
	// given
	s := SetupStateTest(t, itest.WithPostgres)
	defer s.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	// gets only expired challenges
	s.EntropySource = &ext.StableSource{Time: itest.TimeNow}
	challenges, err := s.GetChallengesByParticipant(ctx, ChallengeKey{int64(5), -1})
	require.NoError(t, err)

	// then
	assert.Equal(t, []ChallengeEntity{TestChallengeEntities[2], TestChallengeEntities[3]}, challenges)
}

func TestDeleteExpiredChallenges(t *testing.T) {
	// given
	s := SetupStateTest(t, itest.UseTxn, itest.WithPostgres)
	defer s.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// when
	// gets only expired challenges
	s.EntropySource = &ext.StableSource{Time: itest.TimeNow}
	require.NoError(t, s.DeleteExpiredChallenges(ctx, 5))

	// gets ALL challenges to check that we deleted expired challenges
	s.EntropySource = &ext.StableSource{Time: time.Unix(0, 0)}
	challengesDel, err := s.GetChallengesByParticipant(ctx, ChallengeKey{int64(5), -1})
	require.NoError(t, err)

	// then
	assert.Equal(t, []ChallengeEntity{TestChallengeEntities[2], TestChallengeEntities[3]}, challengesDel)
}

func TestDeleteChallenge(t *testing.T) {
	// given
	s := SetupStateTest(t, itest.UseTxn, itest.WithPostgres)
	defer s.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s.EntropySource = &ext.StableSource{Time: itest.TimeNow}

	key := ChallengeKey{ChallengerID: 1, ChallengeeID: 2}

	// when
	challengeBefore, err := s.Query().SelectChallenge(ctx, db.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	dr, err := s.DeleteChallenge(ctx, key)
	require.NoError(t, err)

	_, errAfterDelete := s.Query().SelectChallenge(ctx, db.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	// then
	challenge := db.Challenge{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, StartColor: "RANDOM", MadeOn: pgtype.Timestamptz{Valid: true, Time: itest.TimeNow.Local()}, Mode: "TIMED_3+2"}
	assert.Equal(t, challenge, challengeBefore)
	assert.Error(t, pgx.ErrNoRows, errAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, Mode: ModeTimed3Plus2, FirstColor: Random}, dr)
}
