package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/config"
	"hexchess-svc/database"
	"hexchess-svc/game"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/queue/producers"
	"hexchess-svc/replay"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

type RedisConsumerConfig struct {
	Context     context.Context
	Database    database.PostgreSQL
	Redis       cache.Redis
	Broadcaster publishers.Broadcaster
	RiverClient database.RiverClientAPI

	FeatureFlagConfig config.FeatureFlags

	AckChan       chan redis.XMessage
	BlockDuration time.Duration
}

func StartRedisConsumers(config RedisConsumerConfig) func() {
	slog.Info("start redis consumers", "config", config)

	if config.Context == nil {
		config.Context = context.Background()
	}

	finishGameWorker := NewFinishedGameWorker(config.Database, config.Redis, config.RiverClient, config.Broadcaster)
	updateGameWorker := NewUpdtGameMetadataWorker(config.Database, entropy.RealSource{}, config.Broadcaster)

	redisClient := config.Redis.ConsumerClient
	ackChan := config.AckChan

	streamConfigs := []StreamConfig{
		{
			Context:       config.Context,
			StreamKey:     cache.Constants.FinishGameStreamKey,
			ConsumerGroup: cache.Constants.FinishGameConsumerGroup,
			PollCount:     8,
			PartitionKeys: cache.PartitionsPerGameJobConsumer,
			Redis:         redisClient,
			ConsumeFn:     finishGameWorker.Handle,
			AckChan:       ackChan,
			BlockDuration: config.BlockDuration,
		},
		{
			Context:       config.Context,
			StreamKey:     cache.Constants.UpdtGameMetaStreamKey,
			ConsumerGroup: cache.Constants.UpdtGameMetaConsumerGroup,
			PollCount:     8,
			PartitionKeys: cache.PartitionsPerGameJobConsumer,
			Redis:         redisClient,
			ConsumeFn:     updateGameWorker.Handle,
			AckChan:       ackChan,
			BlockDuration: config.BlockDuration,
		},
	}

	var totalPartitionsCount int
	for _, cfg := range streamConfigs {
		totalPartitionsCount += len(cfg.PartitionKeys)
	}

	// assert: prevent consumer from creating more partitions than is allowed by the client
	// if consumer count exceeds consumer connection pool size certain events get dropped randomly because each stream takes a persistent TCP connection
	allowedPartitionCount := config.Redis.TotalPartitionCount
	if totalPartitionsCount > allowedPartitionCount {
		slog.Error("consumer partition count exceeds redis client allowed partition count - failing fast", "consumerTotalPartitionsCount", totalPartitionsCount, "allowedPartitionCount", allowedPartitionCount)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	for _, cfg := range streamConfigs {
		consumer := NewStreamConsumer(cfg)
		wg.Go(consumer.Consume)
	}

	return func() {
		wg.Wait()
		slog.Info("received shutdown signal - all consumers are stopped")
	}
}

type FinishedGameWorker struct {
	service     *game.GameOverService
	replay      *replay.ReplayService
	broadcaster publishers.Broadcaster
}

func NewFinishedGameWorker(
	database database.PostgreSQL,
	redis cache.Redis,
	riverClient database.RiverClientAPI,
	broadcaster publishers.Broadcaster,
) *FinishedGameWorker {
	return &FinishedGameWorker{
		service: game.NewGameoverService(
			database,
			redis,
			producers.NewRiverProducer(riverClient),
			broadcaster,
			game.NewChessRepoService(redis),
		),
		replay:      replay.NewReplayService(database),
		broadcaster: broadcaster,
	}
}

func (w FinishedGameWorker) Handle(ctx context.Context, bytes []byte) error {
	var event game.FinishGameEvent
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling finished game event", "gameID", event.GameID)

	// insert the finished game into the system of record
	result, err := w.service.HandleFinishedGame(ctx, event)
	if err != nil {
		return serrors.New("insert finished game failed", err)
	}

	// notify any subscribers of the game that replay has been created (game has ended)
	// note: replay is selected outside InsertFinishedGame to avoid holding locks. this involves performing more disk IO.
	gameReplay, err := w.replay.GetReplay(ctx, result.ReplayID)
	if err != nil {
		return serrors.New("get replay by id", err, "replayID", result.ReplayID)
	}
	w.broadcaster.BroadcastGamesEvent(ctx, game.SerializeReplayOutput(game.ReplayGameOutput{GameID: result.GameID, Replay: gameReplay}))

	return nil
}

type UpdtGameMetadataWorker struct {
	service *game.ChessMetadataService
}

func NewUpdtGameMetadataWorker(database database.PostgreSQL, entropy entropy.Generator, broadcaster publishers.Broadcaster) *UpdtGameMetadataWorker {
	return &UpdtGameMetadataWorker{
		service: game.NewChessMetadataService(database, entropy, broadcaster),
	}
}

func (w UpdtGameMetadataWorker) Handle(ctx context.Context, bytes []byte) error {
	var event game.UpdtGameMetadataEvent
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return w.service.UpdateGameMetadata(ctx, event)
}
