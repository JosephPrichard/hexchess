package consumers

import (
	"context"
	"hexchess-svc/queue"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"runtime/debug"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type RiverConsumerSetup struct {
	Ctx         context.Context
	PgxPool     *pgxpool.Pool
	RiverConfig *river.Config
	Services    *svc.HexchessServices
}

const RiverQueueMaxWorkers = 100

func StartRiverConsumers(setup RiverConsumerSetup) {
	if setup.Ctx == nil {
		setup.Ctx = context.Background()
	}
	if setup.RiverConfig == nil {
		setup.RiverConfig = &river.Config{}
	}

	setup.RiverConfig.Logger = slog.Default()
	setup.RiverConfig.Workers = river.NewWorkers()
	setup.RiverConfig.Queues = map[string]river.QueueConfig{
		river.QueueDefault: {MaxWorkers: RiverQueueMaxWorkers},
	}
	setup.RiverConfig.ErrorHandler = &defaultErrorHandler{}

	river.AddWorker(setup.RiverConfig.Workers, &AdvanceTournamentWorker{services: setup.Services})

	riverClient, err := river.NewClient(riverpgxv5.New(setup.PgxPool), setup.RiverConfig)
	if err != nil {
		logutil.Fatal("create river queue client", err)
	}
	if err := riverClient.Start(setup.Ctx); err != nil {
		logutil.Fatal("start river client consumers", err)
	}

	slog.Info("start river consumers")
}

type defaultErrorHandler struct{}

func (*defaultErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	slog.WarnContext(ctx, "river queue event error", "error", err, "job", job)

	if errutil.IsType[NonRetryableQueueError](err) {
		// non-retryable errors are canceled so they don't stay in the queue
		return &river.ErrorHandlerResult{SetCancelled: true}
	}
	return nil
}

func (*defaultErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	slog.ErrorContext(ctx, "panic: river queue event", "err", panicVal, "riverTrace", trace, "job", job, "stack", string(debug.Stack()))
	return nil
}

type AdvanceTournamentWorker struct {
	river.WorkerDefaults[queue.AdvanceTournamentJob]
	services *svc.HexchessServices
}

func (w *AdvanceTournamentWorker) Work(ctx context.Context, job *river.Job[queue.AdvanceTournamentJob]) error {
	slog.InfoContext(ctx, "begin tournament advance event", "job", job.Args)

	gameIDs, err := w.services.ProgressTournament(ctx, job.Args.TournamentKey, job.Args.EventID)
	if errutil.IsType[svc.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce gameIDs", "gameIDs", gameIDs)
	return nil
}
