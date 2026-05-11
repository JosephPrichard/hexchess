package svc

import (
	"context"
	"errors"
	"hexchess-svc/model"

	"testing"
	"time"

	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		challengerID int64
		challengeeID int64
		wantErr      error
	}{
		{
			name:         "CannotChallengeSelf",
			challengerID: 1,
			challengeeID: 1,
			wantErr:      ErrSelfChallenge,
		},
		{
			name:         "InvalidUser",
			challengerID: 9000,
			challengeeID: 1,
			wantErr:      ErrInvalidChallengeMember,
		},
		{
			name:         "DuplicateChallenge",
			challengerID: 1,
			challengeeID: 2,
			wantErr:      ErrDuplicateChallenge,
		},
		{
			name:         "ValidChallenge",
			challengerID: 2,
			challengeeID: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
			defer services.Close()

			ctx := context.WithValue(t.Context(), logutil.Trace, tt.name)

			_, err := services.InsertChallenge(ctx, ChallengeInst{
				ChallengerID: tt.challengerID,
				ChallengeeID: tt.challengeeID,
				Mode:         model.ModeCorrespondence7,
				StartColor:   model.Random,
			})

			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestMapChallengeInsertErr(t *testing.T) {
	tests := []struct {
		name  string
		input error
		want  error
	}{
		{
			name:  "UniqueViolationReturnsErrDuplicateChallenge",
			input: &pgconn.PgError{Code: db.ErrPgUniqueViolation},
			want:  ErrDuplicateChallenge,
		},
		{
			name:  "ForeignKeyViolationReturnsErrParticipantConflict",
			input: &pgconn.PgError{Code: db.ErrPgForeignKeyViolation},
			want:  ErrInvalidChallengeMember,
		},
		{
			name:  "CheckViolationReturnsErrParticipantConflict",
			input: &pgconn.PgError{Code: db.ErrPgCheckViolation},
			want:  ErrInvalidChallengeMember,
		},
		{
			name:  "UnrecognisedErrorIsReturnedAsIs",
			input: errors.New("some unexpected db error"),
			want:  nil,
		},
		{
			name:  "UnknownPgErrorCodeIsReturnedAsIs",
			input: &pgconn.PgError{Code: "99999"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapChallengeInsertErr(tt.input)
			require.Equal(t, tt.want, err)
		})
	}
}

func TestGetChallengesByParticipant(t *testing.T) {
	t.Parallel()

	// gets only expired challenges
	mocks := Mocks{Entropy: &StableEntropySource{CurrTime: itest.TimeNow}}

	services, _ := SetupServicesTest(t, mocks, itest.ROPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	challenges, err := services.GetChallengesByParticipant(ctx, ChallengeKey{int64(5), -1})
	require.NoError(t, err)

	assert.Equal(t, []model.Challenge{itest.TestChallenge[2], itest.TestChallenge[3]}, challenges)
}

func TestDeleteExpiredChallenges(t *testing.T) {
	t.Parallel()

	services, _ := SetupServicesTest(t, Mocks{}, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	// gets only expired challenges
	services.entropy = &StableEntropySource{CurrTime: itest.TimeNow}
	require.NoError(t, services.DeleteExpiredChallenges(ctx, 5))

	// gets ALL challenges to check that we deleted expired challenges
	services.entropy = &StableEntropySource{CurrTime: time.Unix(0, 0)}
	challengesDel, err := services.GetChallengesByParticipant(ctx, ChallengeKey{int64(5), -1})
	require.NoError(t, err)

	assert.Equal(t, []model.Challenge{itest.TestChallenge[2], itest.TestChallenge[3]}, challengesDel)
}

func TestDeleteChallenge(t *testing.T) {
	t.Parallel()

	mocks := Mocks{Entropy: &StableEntropySource{CurrTime: itest.TimeNow}}

	services, _ := SetupServicesTest(t, mocks, itest.RWPostgres)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	key := ChallengeKey{ChallengerID: 1, ChallengeeID: 2}

	challengeBefore, err := services.db.Querier().SelectChallenge(ctx, sqlc.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	dr, err := services.DeleteChallenge(ctx, key)
	require.NoError(t, err)

	_, errAfterDelete := services.db.Querier().SelectChallenge(ctx, sqlc.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	challenge := sqlc.Challenge{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, StartColor: "RANDOM", MadeOn: pgtype.Timestamptz{Valid: true, Time: itest.TimeNow.Local()}, Mode: "TIMED_3+2"}
	assert.Equal(t, challenge, challengeBefore)
	assert.Error(t, pgx.ErrNoRows, errAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, Mode: model.ModeTimed3Plus2, FirstColor: model.Random}, dr)
}
