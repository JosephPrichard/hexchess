package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/serrors"

	"hexchess-svc/model"
	"log/slog"

	"github.com/bytedance/sonic"
)

// TotalPartitionCount is the total number of partitions created by ALL redis consumers
// it can be computed in the function, but it is easier and clearer to keep it hardcoded.
var (
	TotalPartitionCount        = 2 * GameConsumerPartitionCount
	GameConsumerPartitionCount = len(GameConsumerPartitions)
	GameConsumerPartitions     = model.GameIDPartitions()
)

type RedisConsumerConfig struct {
	Database    database.Database
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
	RiverClient database.RiverClientAPI
}

func StartRedisConsumers(config RedisConsumerConfig) {
	slog.Info("start redis consumers", "config", config)

	finishGameWorker := NewFinishedGameWorker(config.Database, config.Redis, config.RiverClient, config.Broadcaster)
	updtGameWorker := NewUpdtGameMetadataWorker(config.Database, entropy.RealSource{}, config.Broadcaster)

	redisClient := config.Redis.ConsumerClient
	metricQuerier := config.Database.QuerierMutator()

	streamConfigs := []StreamConfig{
		{
			StreamKey:     cache.Constants.FinishGameStreamKey,
			ConsumerGroup: cache.Constants.FinishGameConsumerGroup,
			PollCount:     8,
			PartitionKeys: GameConsumerPartitions,

			Redis:          redisClient,
			MetricsQuerier: metricQuerier,

			ConsumeFn: finishGameWorker.Handle,
		},
		{
			StreamKey:     cache.Constants.UpdtGameMetaStreamKey,
			ConsumerGroup: cache.Constants.UpdtGameMetaConsumerGroup,
			PollCount:     8,
			PartitionKeys: GameConsumerPartitions,

			Redis:          redisClient,
			MetricsQuerier: metricQuerier,

			ConsumeFn: updtGameWorker.Handle,
		},
	}

	for _, config := range streamConfigs {
		consumer := NewStreamConsumer(config)
		go consumer.Consume()
	}
}

type FinishedGameWorker struct {
	services    *service.GameOverService
	replay      *service.ReplayService
	broadcaster pubsub.Broadcaster
}

func NewFinishedGameWorker(
	database database.Database,
	redis cache.Redis,
	riverClient database.RiverClientAPI,
	broadcaster pubsub.Broadcaster,
) *FinishedGameWorker {
	return &FinishedGameWorker{
		services: service.NewGameoverService(
			database,
			redis,
			producers.NewRiverProducer(riverClient),
			broadcaster,
			service.NewChessRepoService(redis),
		),
		replay:      service.NewReplayService(database),
		broadcaster: broadcaster,
	}
}

func (w FinishedGameWorker) Handle(ctx context.Context, bytes []byte) error {
	var event model.FinishGameEvent
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling finished game event", "gameID", event.GameID)

	// insert the finished game into the system of record
	result, err := w.services.HandleFinishedGame(ctx, event)
	if err != nil {
		return serrors.New("insert finished game failed", err)
	}

	// notify any subscribers of the game that replay has been created (game has ended)
	// note(Joseph): replay is selected outside InsertFinishedGame to avoid holding locks. this involves performing more disk IO.
	gameReplay, err := w.replay.GetReplay(ctx, result.ReplayID)
	if err != nil {
		return serrors.New("get replay by id", err, "replayID", result.ReplayID)
	}
	w.broadcaster.BroadcastGamesEvent(ctx, model.SerializeReplayOutput(model.ReplayGameOutput{GameID: result.GameID, Replay: gameReplay}))

	return nil
}

type UpdtGameMetadataWorker struct {
	services *service.ChessMetaService
}

func NewUpdtGameMetadataWorker(database database.Database, entropy entropy.Generator, broadcaster pubsub.Broadcaster) *UpdtGameMetadataWorker {
	return &UpdtGameMetadataWorker{
		services: service.NewChessMetaService(database, entropy, broadcaster),
	}
}

func (w UpdtGameMetadataWorker) Handle(ctx context.Context, bytes []byte) error {
	var event model.UpdtGameMetadataEvent
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return w.services.UpdateGameMetadata(ctx, event)
}
