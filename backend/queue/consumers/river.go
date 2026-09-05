package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/service/tournament"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"runtime/debug"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type RiverConsumerConfig struct {
	PGXPool     *pgxpool.Pool
	Database    database.Database
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
}

const RiverQueueMaxWorkers = 100

func StartRiverConsumers(config RiverConsumerConfig) {
	slog.Info("start redis consumers", "config", config)

	riverConfig := &river.Config{
		Logger:  slog.Default(),
		Workers: river.NewWorkers(),
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: RiverQueueMaxWorkers},
		},
		ErrorHandler: &defaultErrorHandler{},
	}
	river.AddWorker(riverConfig.Workers, NewAdvanceTournamentWorker(config.Database, config.Redis, config.Broadcaster))

	riverConsumerClient, err := river.NewClient(riverpgxv5.New(config.PGXPool), riverConfig)
	if err != nil {
		slogutil.Fatal("create river queue client", err)
	}
	if err := riverConsumerClient.Start(context.Background()); err != nil {
		slogutil.Fatal("start river client consumers", err)
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
	orchestrator *tournament.TournamentAdvanceService
}

func NewAdvanceTournamentWorker(
	database database.Database,
	redis cache.Redis,
	broadcaster pubsub.Broadcaster,
) *AdvanceTournamentWorker {
	return &AdvanceTournamentWorker{
		orchestrator: tournament.NewTournamentAdvanceService(
			database,
			user.NewUserService(database),
			gameplay.NewGameCreateService(
				redis,
				gamestate.NewChessRepoService(redis),
			),
			broadcaster,
		),
	}
}

func (w *AdvanceTournamentWorker) Work(ctx context.Context, job *river.Job[queue.AdvanceTournamentJob]) error {
	slog.InfoContext(ctx, "begin tournament advance event", "job", job.Args)

	gameIDs, err := w.orchestrator.AdvanceTournament(ctx, job.Args.TournamentKey, job.Args.EventID)
	if errutil.IsType[tournament.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce gameIDs", "gameIDs", gameIDs)
	return nil
}
