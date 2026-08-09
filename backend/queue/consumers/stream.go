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

type RedisConsumerSetup struct {
	Redis       cache.Redis
	Database    database.Database
	Broadcaster pubsub.Broadcaster
	RiverClient database.RiverClientAPI
}

func StartRedisConsumers(database database.Database, redis cache.Redis, broadcaster pubsub.Broadcaster, riverClient database.RiverClientAPI) {
	finishGameWorker := NewFinishedGameWorker(database, redis, riverClient, broadcaster)
	updtGameWorker := NewUpdtGameMetadataWorker(database, entropy.RealSource{}, broadcaster)

	redisClient := redis.ConsumerClient
	metricQuerier := database.QuerierMutator()

	finishGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     redis.FinishGameStreamKey,
		ConsumerGroup: redis.FinishGameConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: finishGameWorker.Handle,
	})
	go finishGameConsumer.Consume()

	updtGameConsumer := NewStreamConsumer(StreamConfig{
		StreamKey:     redis.UpdtGameMetaStreamKey,
		ConsumerGroup: redis.UpdtGameMetaConsumerGroup,
		PollCount:     8,
		PartitionKeys: GameConsumerPartitions,

		Redis:          redisClient,
		MetricsQuerier: metricQuerier,

		ConsumeFn: updtGameWorker.Handle,
	})
	go updtGameConsumer.Consume()
}

type FinishedGameWorker struct {
	services    *gameplay.GameOverService
	replay      *replay.ReplayService
	broadcaster pubsub.Broadcaster
}

func NewFinishedGameWorker(
	database database.Database,
	redis cache.Redis,
	riverClient database.RiverClientAPI,
	broadcaster pubsub.Broadcaster,
) *FinishedGameWorker {
	return &FinishedGameWorker{
		services: gameplay.NewGameoverService(
			database,
			redis,
			producers.NewRiverProducer(riverClient),
			broadcaster,
		),
		replay:      replay.NewReplayService(database),
		broadcaster: broadcaster,
	}
}

func (w FinishedGameWorker) Handle(ctx context.Context, bytes []byte) error {
	var event model.FinishedGame
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling finished game event", "gameID", event.GameID)

	// step 1: insert the finished game into the system of record
	result, err := w.services.InsertFinishedGame(ctx, event)
	if err != nil {
		return serrors.New("insert finished game failed", err)
	}

	// step 2: notify any subscribers of the game that replay has been created (game has ended)
	// note(Joseph): replay is selected outside InsertFinishedGame to avoid holding locks. this involves performing more disk IO.
	gameReplay, err := w.replay.GetReplay(ctx, result.ReplayID)
	if err != nil {
		return serrors.New("get replay by id", err, "replayID", result.ReplayID)
	}
	w.broadcaster.BroadcastGamesEvent(ctx, model.SerializeReplayOutput(model.ReplayGameOutput{GameID: result.GameID, Replay: gameReplay}))

	return nil
}

type UpdtGameMetadataWorker struct {
	services *gamestate.ChessMetaService
}

func NewUpdtGameMetadataWorker(database database.Database, entropy entropy.Generator, broadcaster pubsub.Broadcaster) *UpdtGameMetadataWorker {
	return &UpdtGameMetadataWorker{
		services: gamestate.NewChessMetaService(database, entropy, broadcaster),
	}
}

func (w UpdtGameMetadataWorker) Handle(ctx context.Context, bytes []byte) error {
	var event model.GameMetadataUpdt
	if err := sonic.Unmarshal(bytes, &event); err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return w.services.UpdateGameMetadata(ctx, event)
}
