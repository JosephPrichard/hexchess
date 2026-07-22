package consumers

import (
	"hexchess-svc/db/primarydb"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	"hexchess-svc/queue/producers"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdbtest"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivershared/util/testutil"
	"github.com/riverqueue/river/rivertest"
	"github.com/stretchr/testify/require"
)

func initRiverTest(t *testing.T, services *svc.HexchessServices) (*river.Client[pgx.Tx], *pgxpool.Pool, *river.Config) {
	riverPool, err := pgxpool.New(t.Context(), "postgres://localhost:5432/river_test?pool_max_conns=15&sslmode=disable")
	if err != nil {
		t.Fatal("failed to create mock river pool", err)
	}
	defer riverPool.Close()

	config := &river.Config{}

	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if riverPool != nil {
		config.Schema = riverdbtest.TestSchema(t.Context(), testutil.PanicTB(), riverpgxv5.New(riverPool), nil)
	}
	config.TestOnly = true

	workers := river.NewWorkers()
	river.AddWorker(workers, &AdvanceTournamentWorker{services: services})

	riverClient, err := river.NewClient(riverpgxv5.New(riverPool), config)
	if err != nil {
		logutil.Fatal("create river queue client", err)
	}

	return riverClient, riverPool, config
}

var sqlcTournamentMatchCmpOpts = cmpopts.IgnoreFields(primarydb.TournamentMatch{}, "Ordering", "CreatedOn", "GameID")

func TestHandleAdvanceTournamentEvent(t *testing.T) {
	// NON-parallel, rivertest does not support parallel tests
	ctx := t.Context()

	testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres, itest.Redis)
	defer testinfra.Close()

	services := svc.NewHexchessServices(svc.SetupService{
		PrimaryDB:   testinfra.PrimaryDB,
		Redis:       testinfra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(testinfra.Redis),
	})

	riverClient, riverPool, riverConfig := initRiverTest(t, services)

	tournamentKey := itest.Tournament2ScheduledKnockoutKey

	producer := producers.NewRiverProducer(riverClient)

	testingTxn, err := riverPool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin mock river tx: %v", err)
	}
	defer testingTxn.Rollback(ctx)

	err = producer.ProduceAdvanceTournament(ctx, testingTxn, producers.AdvanceTournamentArgs{TournamentKey: tournamentKey})
	require.NoError(t, err)

	_ = rivertest.RequireInsertedTx[*riverpgxv5.Driver](ctx, t, testingTxn, &queue.AdvanceTournamentJob{}, &rivertest.RequireInsertedOpts{
		Priority: 1,
		Queue:    river.QueueDefault,
		Schema:   riverConfig.Schema,
	})

	//wantMatches := []primarydb.TournamentMatch{
	//	// tournament has 2 rounds with join order of [1,2,3,4], so starting the tournament creates 2 rounds wso the matches go 1-2, 3-4
	//	{
	//		TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
	//		Round:         1,
	//		WhiteID:       3,
	//		BlackID:       4,
	//	},
	//	{
	//		TournamentKey: pgtype.UUID{Bytes: itest.Tournament2ScheduledKnockoutKey, Valid: true},
	//		Round:         1,
	//		WhiteID:       5,
	//		BlackID:       6,
	//	},
	//}
	//
	//matches, err := testinfra.PrimaryQuerier.SelectMatches(ctx, pgtype.UUID{Bytes: tournamentKey, Valid: true})
	//require.NoError(t, err)
	//testutil.Equal(t, wantMatches, matches, sqlcTournamentMatchCmpOpts)
}
