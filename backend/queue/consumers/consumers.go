package consumers

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log/slog"
	"time"
)

type SetupConsumers struct {
	Ctx      context.Context
	Services *svc.HexchessServices
	Postgres db.Database
	Redis    db.Redis
}

func StartConsumers(setup SetupConsumers) {
	eventGateway := EventGateway{services: setup.Services}
	consumers := []Consumer{
		&PostgresConsumer{
			ctx: setup.Ctx,
			pdb: setup.Postgres,

			EventKind:    sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
			PollInterval: 1 * time.Second,
			PollCount:    32,
			fn:           eventGateway.HandleAdvanceTournamentEvent,
		},
		&RedisConsumer{
			ctx:   setup.Ctx,
			redis: setup.Redis.GameStore,

			StreamKey:     setup.Redis.FinishGameStreamKey,
			ConsumerGroup: setup.Redis.FinishGameConsumerGroup,
			Concurrency:   8,
			PartitionKeys: model.GameIDPartitions(),

			fn: eventGateway.HandleFinishedGameEvent,
		},
		&RedisConsumer{
			ctx:   setup.Ctx,
			redis: setup.Redis.GameStore,

			StreamKey:     setup.Redis.UpdtGameMetaStreamKey,
			ConsumerGroup: setup.Redis.UpdtGameMetaConsumerGroup,
			Concurrency:   8,
			PartitionKeys: model.GameIDPartitions(),

			fn: eventGateway.HandleUpdtGameEvent,
		},
	}

	slog.InfoContext(setup.Ctx, "starting consumers", "consumers", consumers)

	for _, consumer := range consumers {
		go consumer.Consume()
		slog.InfoContext(setup.Ctx, "started consumer", "consumer", consumer)
	}
}

type Consumer interface {
	Consume()
}

type ConsumeFunc func(ctx context.Context, bytes []byte) error

type EventGateway struct {
	services *svc.HexchessServices
}

func (gateway EventGateway) HandleFinishedGameEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalFinishedGame(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "handling finished game event", "event", event)
	return gateway.services.InsertFinishedGame(ctx, event)
}

func (gateway EventGateway) HandleUpdtGameEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalGameMetadataUpdt(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "handling update game metadata event", "event", event)
	return gateway.services.UpdateGameMetadata(ctx, event)
}

func (gateway EventGateway) HandleAdvanceTournamentEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalAdvanceTournamentEvent(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "begin tournament advance event", "event", event)

	_, err = gateway.services.AdvanceTournament(ctx, event.TournamentKey, event.EventID)
	if errutil.IsType[svc.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	}
	return err
}
