package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/db"

	"hexchess-svc/model"
	svc "hexchess-svc/service"
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

type RedisConsumerSetup struct {
	Redis    cache.Redis
	Database db.Database
	Services *svc.HexchessServices
}

func StartRedisConsumers(setup RedisConsumerSetup) {
	finishGameHandler := FinishedGameHandler{services: setup.Services}
	updtGameHandler := UpdtGameMetadataHandler{services: setup.Services}

	redisClient := setup.Redis.ConsumerClient
	metricQuerier := setup.Database.QuerierMutator()

	finishGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     setup.Redis.FinishGameStreamKey,
		ConsumerGroup: setup.Redis.FinishGameConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: finishGameHandler.Handle,
	})
	go finishGameConsumer.Consume()

	updtGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     setup.Redis.UpdtGameMetaStreamKey,
		ConsumerGroup: setup.Redis.UpdtGameMetaConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: updtGameHandler.Handle,
	})
	go updtGameConsumer.Consume()
}

type FinishedGameHandler struct {
	services *svc.HexchessServices
}

func (handler FinishedGameHandler) Handle(ctx context.Context, bytes []byte) error {
	var event model.FinishedGame
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling finished game event", "gameID", event.GameID)

	return handler.services.InsertFinishedGame(ctx, event)
}

type UpdtGameMetadataHandler struct {
	services *svc.HexchessServices
}

func (handler UpdtGameMetadataHandler) Handle(ctx context.Context, bytes []byte) error {
	var event model.GameMetadataUpdt
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return handler.services.UpdateGameMetadata(ctx, event)
}
