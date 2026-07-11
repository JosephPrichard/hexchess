package consumers

import (
	"context"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"hexchess-svc/utils/errutil"
	"log/slog"
)

type EventGateway struct {
	services *svc.HexchessServices
}

func (gateway EventGateway) HandleFinishedGameEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalFinishedGame(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling finished game event", "gameID", event.GameID)

	return gateway.services.InsertFinishedGame(ctx, event)
}

func (gateway EventGateway) HandleUpdtGameEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalGameMetadataUpdt(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "handling update game metadata event", "gameID", event.GameID)

	return gateway.services.UpdateGameMetadata(ctx, event)
}

func (gateway EventGateway) HandleAdvanceTournamentEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalAdvanceTournamentEvent(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}

	slog.InfoContext(ctx, "begin tournament advance event", "event", event)

	gameIDs, err := gateway.services.AdvanceTournament(ctx, event.TournamentKey, event.EventID)
	if errutil.IsType[svc.MatchInvariantError](err) {
		return NonRetryableQueueError{Err: err}
	} else if err != nil {
		return err
	}

	slog.InfoContext(ctx, "advanced tournaments to produce game IDs", "gameIDs", gameIDs)
	return nil
}
