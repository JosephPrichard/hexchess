package consumers

import (
	"context"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log/slog"
)

func HandleFinishedGameEvent(services svc.HexchessAPI) RedisHandlerFunc {
	return func(ctx context.Context, eventData string) error {
		event, err := model.UnmarshalFinishedGame([]byte(eventData))
		if err != nil {
			return NonRetryableQueueError{Err: err}
		}
		slog.InfoContext(ctx, "handling finished game event", "event", event)
		return services.InsertFinishedGame(ctx, event)
	}
}

func HandleUpdateGameMetadataEvent(services svc.HexchessAPI) RedisHandlerFunc {
	return func(ctx context.Context, eventData string) error {
		event, err := model.UnmarshalGameMetadataUpdt([]byte(eventData))
		if err != nil {
			return NonRetryableQueueError{Err: err}
		}
		slog.InfoContext(ctx, "handling update game metadata event", "event", event)
		return services.UpdateGameMetadata(ctx, event)
	}
}

func HandleAdvanceTournamentEvent(services svc.HexchessAPI) PgHandlerFunc {
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
