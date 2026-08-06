package consumers

import (
	"context"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/service/gameplay"
	"hexchess-svc/service/gamestate"
	"hexchess-svc/service/replay"
	"hexchess-svc/utils/entropy"

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

type RedisConsumerSetup struct {
	Redis       cache.Redis
	Database    database.Database
	Broadcaster pubsub.Broadcaster
	RiverClient producers.RiverClientAPI
}

func StartRedisConsumers(database database.Database, redis cache.Redis, broadcaster pubsub.Broadcaster, riverClient producers.RiverClientAPI) {
	operator := database.Operator()

	finishGameHandler := FinishedGameHandler{services: gameplay.NewGameoverService(
		operator,
		redis,
		producers.NewRiverProducer(riverClient),
		broadcaster,
		replay.NewReplayService(operator),
	)}
	updtGameHandler := UpdtGameMetadataHandler{
		services: gamestate.NewChessMetaService(operator, entropy.RealSource{}, broadcaster),
	}

	redisClient := redis.ConsumerClient
	metricQuerier := database.QuerierMutator()

	finishGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     redis.FinishGameStreamKey,
		ConsumerGroup: redis.FinishGameConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: finishGameHandler.Handle,
	})
	go finishGameConsumer.Consume()

	updtGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     redis.UpdtGameMetaStreamKey,
		ConsumerGroup: redis.UpdtGameMetaConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: updtGameHandler.Handle,
	})
	go updtGameConsumer.Consume()
}

type FinishedGameHandler struct {
	services *gameplay.GameOverService
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
	services *gamestate.ChessMetaService
}

func (handler UpdtGameMetadataHandler) Handle(ctx context.Context, bytes []byte) error {
	var event model.GameMetadataUpdt
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return handler.services.UpdateGameMetadata(ctx, event)
}
