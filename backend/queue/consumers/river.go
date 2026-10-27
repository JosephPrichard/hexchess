package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/config"
	"hexchess-svc/database"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue"
	"hexchess-svc/service"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"runtime/debug"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type RiverConsumerConfig struct {
	PGXPool           *pgxpool.Pool
	Database          database.Database
	Redis             cache.Redis
	EsClient          *elasticsearch.TypedClient
	Broadcaster       pubsub.Broadcaster
	FeatureFlagConfig config.FeatureFlags
}

const RiverQueueMaxWorkers = 100

func StartRiverConsumers(config RiverConsumerConfig) {
	slog.Info("start redis consumers", "config", config)

	// initialize services
	advanceTournamentWorker := NewAdvanceTournamentWorker(config.Database, config.Redis, config.Broadcaster)
	updateUserWorker := NewUpdateUserWorker(config.Database, config.EsClient)

	// override feature flags
	updateUserWorker.service.FeatureFlags = config.FeatureFlagConfig

	// configure and connect to river queue
	riverConfig := &river.Config{
		Logger:  slog.Default(),
		Workers: river.NewWorkers(),
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: RiverQueueMaxWorkers},
		},
		ErrorHandler: &defaultErrorHandler{},
		Plugins: []rivertype.Plugin{
			&riverQueueMiddleware{},
		},
	}
	river.AddWorker(riverConfig.Workers, advanceTournamentWorker)
	river.AddWorker(riverConfig.Workers, updateUserWorker)

	riverConsumerClient, err := river.NewClient(riverpgxv5.New(config.PGXPool), riverConfig)
	if err != nil {
		slogutil.Fatal("create river queue client", err)
	}
	if err := riverConsumerClient.Start(context.Background()); err != nil {
		slogutil.Fatal("start river client consumers", err)
	}

	slog.Info("start river consumers")
}

type riverQueueMiddleware struct {
	river.MiddlewareDefaults
}

func (m *riverQueueMiddleware) Work(
	ctx context.Context,
	job *rivertype.JobRow,
	doInner func(ctx context.Context) error,
) error {
	ctx = slogutil.TracedContext(ctx, uuid.NewString())

	slog.InfoContext(ctx, "riverqueue middleware: before working job", "jobID", job.ID, "kind", job.Kind)

	err := doInner(ctx)

	if err != nil {
		slog.ErrorContext(ctx, "riverqueue middleware: job failed", "jobID", job.ID, "error", err)
	} else {
		slog.InfoContext(ctx, "riverqueue middleware: job succeeded", "jobID", job.ID)
	}

	return err
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
	service *service.TournamentAdvancer
}

func NewAdvanceTournamentWorker(
	database database.Database,
	redis cache.Redis,
	broadcaster pubsub.Broadcaster,
) *AdvanceTournamentWorker {
	return &AdvanceTournamentWorker{
		service: service.NewTournamentAdvancer(
			database,
			broadcaster,
			service.NewGameCreateService(
				redis,
				service.NewChessRepoService(redis),
			),
		),
	}
}

func (w *AdvanceTournamentWorker) Work(ctx context.Context, job *river.Job[queue.AdvanceTournamentJob]) error {
	slog.InfoContext(ctx, "begin tournament advance event", "job", job.Args)

	gameIDs, err := w.service.AdvanceTournament(ctx, job.Args.TournamentKey, job.Args.EventID)
	if errutil.IsType[service.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce gameIDs", "gameIDs", gameIDs)
	return nil
}

type UpdateUserWorker struct {
	river.WorkerDefaults[queue.UpdateUserJob]
	service *service.UserSearchService
}

func NewUpdateUserWorker(
	database database.Database,
	esClient *elasticsearch.TypedClient,
) *UpdateUserWorker {
	return &UpdateUserWorker{
		service: service.NewUserSearchService(database.Querier, esClient),
	}
}

func (w *UpdateUserWorker) Work(ctx context.Context, job *river.Job[queue.UpdateUserJob]) error {
	slog.InfoContext(ctx, "begin update user event", "job", job.Args)

	err := w.service.UpdateUserSearch(ctx, model.UpdateUserEvent(job.Args))
	if err != nil {
		return serrors.New("update user search", err)
	}

	slog.InfoContext(ctx, "update user search index")
	return nil
}
