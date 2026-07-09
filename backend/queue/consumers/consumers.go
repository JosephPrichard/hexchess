package consumers

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log/slog"
	"time"
)

type Consumer interface {
	Consume()
}

type ConsumeFunc func(ctx context.Context, bytes []byte) error

type SetupConsumers struct {
	Ctx      context.Context
	Services *svc.HexchessServices
	Postgres db.Database
	Redis    db.Redis
}

// TotalPartitionCount is the total number of partitions created by ALL redis consumers
// it can be computed in the function, but it is easier and clearer to keep it hardcoded.
var (
	TotalPartitionCount        = 2 * GameConsumerPartitionCount
	GameConsumerPartitionCount = len(GameConsumerPartitions)
	GameConsumerPartitions     = model.GameIDPartitions()
)

func StartConsumers(setup SetupConsumers) {
	if setup.Ctx == nil {
		setup.Ctx = context.Background()
	}

	eventGateway := EventGateway{services: setup.Services}

	consumers := []Consumer{
		&PostgresConsumer{
			ctx:         setup.Ctx,
			pdb:         setup.Postgres,
			consumeFunc: eventGateway.HandleAdvanceTournamentEvent,

			PostgresConfig: PostgresConfig{
				EventKind:    sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
				PollInterval: 1 * time.Second,
				PollCount:    32,
			},
		},
		&RedisConsumer{
			ctx:         setup.Ctx,
			redis:       setup.Redis.Consumer,
			inserter:    setup.Postgres.Querier(),
			consumeFunc: eventGateway.HandleFinishedGameEvent,

			RedisConfig: RedisConfig{
				StreamKey:     setup.Redis.FinishGameStreamKey,
				ConsumerGroup: setup.Redis.FinishGameConsumerGroup,
				PollCount:     8,
				PartitionKeys: GameConsumerPartitions,
			},
		},
		&RedisConsumer{
			ctx:         setup.Ctx,
			redis:       setup.Redis.Consumer,
			inserter:    setup.Postgres.Querier(),
			consumeFunc: eventGateway.HandleUpdtGameEvent,

			RedisConfig: RedisConfig{
				StreamKey:     setup.Redis.UpdtGameMetaStreamKey,
				ConsumerGroup: setup.Redis.UpdtGameMetaConsumerGroup,
				PollCount:     8,
				PartitionKeys: GameConsumerPartitions,
			},
		},
	}

	slog.InfoContext(setup.Ctx, "starting consumers", "consumers", consumers)

	for _, consumer := range consumers {
		go consumer.Consume()
		slog.InfoContext(setup.Ctx, "started consumer", "consumer", consumer)
	}
}
