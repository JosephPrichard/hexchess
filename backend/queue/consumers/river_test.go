package consumers

import (
	"hexchess-svc/db/primarydb"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(primarydb.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestHandleAdvanceTournamentEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		PrimaryDB:   testinfra.PrimaryDB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
	})
	worker := AdvanceTournamentWorker{services: services}

	tournamentKey := itest.Tournament2ScheduledKnockoutKey

	err := worker.Work(ctx, &river.Job[queue.AdvanceTournamentJob]{
		Args: queue.AdvanceTournamentJob{TournamentKey: tournamentKey},
	})
	require.NoError(t, err)

	wantMatches := []primarydb.TournamentMatch{
		// tournament has 2 rounds with join order of [1,2,3,4], so starting the tournament creates 2 rounds wso the matches go 1-2, 3-4
		{
			TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
			Round:         1,
			WhiteID:       3,
			BlackID:       4,
		},
		{
			TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
			Round:         1,
			WhiteID:       5,
			BlackID:       6,
		},
	}

	matches, err := testinfra.PrimaryQuerier.SelectMatches(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	require.NoError(t, err)
	testutil.Equal(t, wantMatches, matches, sqlcTournamentMatchCmpOpts)
}
