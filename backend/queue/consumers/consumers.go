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
	Services svc.HexchessAPI
	Postgres db.DB
	Redis    db.Redis
}

func StartConsumers(setup SetupConsumers) {
	consumers := []Consumer{
		&PostgresConsumer{
			ctx:     setup.Ctx,
			pdb:     setup.Postgres,
			entropy: &svc.RealEntropySource{},

			kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
			pollInterval: 1 * time.Second,
			pollCount:    32,
			fn:           HandleAdvanceTournamentEvent(setup.Services),
		},
		&RedisConsumer{
			ctx:   setup.Ctx,
			redis: setup.Redis.GameStore,

			concurrency:   8,
			streamKey:     setup.Redis.FinishGameStreamKey,
			consumerGroup: setup.Redis.FinishGameConsumerGroup,

			fn: HandleFinishedGameEvent(setup.Services),
		},
		&RedisConsumer{
			ctx:   setup.Ctx,
			redis: setup.Redis.GameStore,

			concurrency:   8,
			streamKey:     setup.Redis.UpdtGameMetaStreamKey,
			consumerGroup: setup.Redis.UpdtGameMetaConsumerGroup,

			fn: HandleUpdtGameEvent(setup.Services),
		},
	}

	slog.InfoContext(setup.Ctx, "starting consumers", "consumers", consumers)

	for _, consumer := range consumers {
		go consumer.Consume()
		slog.InfoContext(setup.Ctx, "started consumer", "consumer", consumer)
	}
}

type Consumer interface {
	Consume() error
}

type ConsumeFunc func(ctx context.Context, bytes []byte) error

func HandleFinishedGameEvent(services svc.HexchessAPI) ConsumeFunc {
	return func(ctx context.Context, bytes []byte) error {
		event, err := model.UnmarshalFinishedGame(bytes)
		if err != nil {
			return NonRetryableQueueError{Err: err}
		}
		slog.InfoContext(ctx, "handling finished game event", "event", event)
		return services.InsertFinishedGame(ctx, event)
	}
}

func HandleUpdtGameEvent(services svc.HexchessAPI) ConsumeFunc {
	return func(ctx context.Context, bytes []byte) error {
		event, err := model.UnmarshalGameMetadataUpdt(bytes)
		if err != nil {
			return NonRetryableQueueError{Err: err}
		}
		slog.InfoContext(ctx, "handling update game metadata event", "event", event)
		return services.UpdateGameMetadata(ctx, event)
	}
}

func HandleAdvanceTournamentEvent(services svc.HexchessAPI) ConsumeFunc {
	return func(ctx context.Context, bytes []byte) error {
		event, err := model.UnmarshalAdvanceTournamentEvent(bytes)
		if err != nil {
			return NonRetryableQueueError{Err: err}
		}
		slog.InfoContext(ctx, "begin tournament advance event", "event", event)

		_, err = services.AdvanceTournament(ctx, event.TournamentKey, event.EventID)
		if errutil.IsType[svc.MatchInvariantError](err) {
			return NonRetryableQueueError{Err: err}
		}
		return err
	}
}
