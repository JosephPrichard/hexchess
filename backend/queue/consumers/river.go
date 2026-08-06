package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/service/tournament"
	"hexchess-svc/service/user"
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
	PgxPool     *pgxpool.Pool
	RiverConfig *river.Config

	Database    database.Database
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
	RiverClient producers.RiverClientAPI
}

const RiverQueueMaxWorkers = 100

func StartRiverConsumers(
	pgxPool *pgxpool.Pool,
	database database.Database,
	redis cache.Redis,
	broadcaster pubsub.Broadcaster,
	riverClient producers.RiverClientAPI,
) {
	ctx := context.Background()

	riverConfig := &river.Config{
		Logger:  slog.Default(),
		Workers: river.NewWorkers(),
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: RiverQueueMaxWorkers},
		},
		ErrorHandler: &defaultErrorHandler{},
	}

	river.AddWorker(riverConfig.Workers, NewAdvanceTournamentWorker(database, redis, broadcaster, riverClient))

	riverConsumerClient, err := river.NewClient(riverpgxv5.New(pgxPool), riverConfig)
	if err != nil {
		logutil.Fatal("create river queue client", err)
	}
	if err := riverConsumerClient.Start(ctx); err != nil {
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
	orchestrator *tournament.TournamentOrchestrator
}

func NewAdvanceTournamentWorker(
	database database.Database,
	redis cache.Redis,
	broadcaster pubsub.Broadcaster,
	riverClient producers.RiverClientAPI,
) *AdvanceTournamentWorker {
	operator := database.Operator()

	return &AdvanceTournamentWorker{
		orchestrator: tournament.NewTournamentOrchestrator(
			tournament.NewTournamentService(
				operator,
				redis,
				producers.NewRiverProducer(riverClient),
			),
			user.NewUserService(operator),
			gameplay.NewGameplayService(
				redis,
				producers.NewStreamProducer(redis),
				gamestate.NewChessRepoService(redis),
			),
			broadcaster,
		),
	}
}

func (w *AdvanceTournamentWorker) Work(ctx context.Context, job *river.Job[queue.AdvanceTournamentJob]) error {
	slog.InfoContext(ctx, "begin tournament advance event", "job", job.Args)

	gameIDs, err := w.orchestrator.ProgressTournament(ctx, job.Args.TournamentKey, job.Args.EventID)
	if errutil.IsType[tournament.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce gameIDs", "gameIDs", gameIDs)
	return nil
}
