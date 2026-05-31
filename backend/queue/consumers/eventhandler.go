package consumers

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	svc "hexchess-svc/service"
	"log/slog"
)

type EventHandler struct {
	services    svc.HexchessAPI
	broadcaster pubsub.BroadcasterAPI
}

func (h EventHandler) HandleAdvanceTournamentEvent(ctx context.Context, bytes []byte) error {
	event, err := model.UnmarshalAdvanceTournamentEvent(bytes)
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "begin tournament advance event", "event", event)

	var errInvariant svc.MatchInvariantError

	_, err = h.services.AdvanceTournament(ctx, event.TournamentKey, event.EventID)
	if errors.As(err, &errInvariant) {
		h.broadcaster.BroadcastTournament(ctx, model.SerializeTournamentError(event.TournamentKey, model.ErrAdvanceTournamentCode))
		return NonRetryableQueueError{Err: errInvariant}
	} else if err != nil {
		return fmt.Errorf("advance tournament: %w", err)
	}

	h.broadcaster.BroadcastTournament(ctx, model.SerializeStartTournament(event.TournamentKey))
	return nil
}

func (h EventHandler) HandleFinishedGameEvent(ctx context.Context, eventData string) error {
	event, err := model.UnmarshalFinishedGame([]byte(eventData))
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "begin finished game event", "event", event)

	err = h.services.InsertFinishedGame(ctx, event)
	return errutil.Guardf(err, "insert finished game event")
}

func (h EventHandler) HandleUpdateGameMetadataEvent(ctx context.Context, eventData string) error {
	event, err := model.UnmarshalGameMetadataUpdt([]byte(eventData))
	if err != nil {
		return NonRetryableQueueError{Err: err}
	}
	slog.InfoContext(ctx, "begin start game event", "event", event)

	err = h.services.UpdateGameMetadata(ctx, event)
	return errutil.Guardf(err, "update game metadata")
}
