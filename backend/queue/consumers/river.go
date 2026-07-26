package consumers

import (
	"context"
	"hexchess-svc/queue"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/logutil"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type RiverConsumerSetup struct {
	PgxPool     *pgxpool.Pool
	RiverConfig *river.Config
	Services    *svc.HexchessServices
}

func StartRiverConsumers(setup RiverConsumerSetup) {
	if setup.RiverConfig == nil {
		setup.RiverConfig = &river.Config{}
	}

	setup.RiverConfig.Logger = slog.Default()
	setup.RiverConfig.Workers = river.NewWorkers()

	river.AddWorker(setup.RiverConfig.Workers, &AdvanceTournamentWorker{services: setup.Services})

	riverClient, err := river.NewClient(riverpgxv5.New(setup.PgxPool), setup.RiverConfig)
	if err != nil {
		logutil.Fatal("create river queue client", err)
	}
	if err := riverClient.Start(context.Background()); err != nil {
		logutil.Fatal("start river client consumers", err)
	}
}

type AdvanceTournamentWorker struct {
	river.WorkerDefaults[queue.AdvanceTournamentJob]
	services *svc.HexchessServices
}

func (w *AdvanceTournamentWorker) Work(ctx context.Context, job *river.Job[queue.AdvanceTournamentJob]) error {
	slog.InfoContext(ctx, "begin tournament advance event", "job", job)

	gameIDs, err := w.services.AdvanceTournament(ctx, job.Args.TournamentKey, job.Args.EventID)
	if errutil.IsType[svc.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce gameIDs", "gameIDs", gameIDs)
	return nil
}
