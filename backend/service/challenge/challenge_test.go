package challenge

import (
	"context"
	"errors"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/entropy"

	"testing"
	"time"

	"hexchess-svc/database"

	"hexchess-svc/itest"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/testutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t alog.TestLogger) (*ChallengeService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	services := NewChallengeService(infra.Database, entropy.RealSource{})

	return services, infra
}

func TestInsertChallenge(t *testing.T) {
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
			services, testinfra := setupTest(t)
			defer testinfra.Close()

			ctx := context.WithValue(t.Context(), alog.Trace, tt.name)

			_, err := services.InsertChallenge(ctx, Inst{
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
			input: &pgconn.PgError{Code: database.ErrPgUniqueViolation},
			want:  ErrDuplicateChallenge,
		},
		{
			name:  "ForeignKeyViolationReturnsErrParticipantConflict",
			input: &pgconn.PgError{Code: database.ErrPgForeignKeyViolation},
			want:  ErrInvalidChallengeMember,
		},
		{
			name:  "CheckViolationReturnsErrParticipantConflict",
			input: &pgconn.PgError{Code: database.ErrPgCheckViolation},
			want:  ErrInvalidChallengeMember,
		},
		{
			name:  "UnrecognisedErrorIsReturnedAsIs",
			input: errors.New("some unexpected transactor error"),
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
	// gets only expired challenges
	services, testinfra := setupTest(t)
	defer testinfra.Close()

	services.entropy = &entropy.StableSource{CurrTime: itest.TimeNow}
	ctx := t.Context()

	challenges, err := services.GetChallengesByParticipant(ctx, Key{int64(5), -1})
	require.NoError(t, err)

	testutil.Equal(t, []model.Challenge{itest.TestChallenge[2], itest.TestChallenge[3]}, challenges)
}

func TestDeleteExpiredChallenges(t *testing.T) {
	services, testinfra := setupTest(t)
	defer testinfra.Close()
	ctx := t.Context()

	// gets only expired challenges
	services.entropy = &entropy.StableSource{CurrTime: itest.TimeNow}

	require.NoError(t, services.DeleteExpiredChallenges(ctx, 5))

	// gets ALL challenges to check that we deleted expired challenges
	services.entropy = &entropy.StableSource{CurrTime: time.Unix(0, 0)}

	challengesDel, err := services.GetChallengesByParticipant(ctx, Key{int64(5), -1})
	require.NoError(t, err)

	assert.Equal(t, []model.Challenge{itest.TestChallenge[2], itest.TestChallenge[3]}, challengesDel)
}

func TestDeleteChallenge(t *testing.T) {
	services, testinfra := setupTest(t)
	defer testinfra.Close()

	services.entropy = &entropy.StableSource{CurrTime: itest.TimeNow}
	ctx := t.Context()

	key := Key{ChallengerID: 1, ChallengeeID: 2}

	challengeBefore, err := testinfra.Querier().SelectChallenge(ctx, query.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	dr, err := services.DeleteChallenge(ctx, key.ChallengerID, key.ChallengeeID)
	require.NoError(t, err)

	_, errAfterDelete := testinfra.Querier().SelectChallenge(ctx, query.SelectChallengeParams{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID})
	require.NoError(t, err)

	challenge := query.Challenge{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, StartColor: "RANDOM", MadeOn: pgtype.Timestamptz{Valid: true, Time: itest.TimeNow.Local()}, Mode: "TIMED_3+2"}
	assert.Equal(t, challenge, challengeBefore)
	assert.Error(t, pgx.ErrNoRows, errAfterDelete)
	assert.Equal(t, DeleteResult{ChallengerID: key.ChallengerID, ChallengeeID: key.ChallengeeID, Mode: model.ModeTimed3Plus2, FirstColor: model.Random}, dr)
}
