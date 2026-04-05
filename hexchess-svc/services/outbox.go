package svc

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"log/slog"
	"sync"
	"time"
)

const (
	OutboxPollInterval = 5 * time.Second
	OutboxPollCount    = 32
)

type handleOutboxEvent func(ctx context.Context, event sqlc.SelectOutboxQueueRow) error

func (svc *Services) handleOutboxEvent(ctx context.Context, event sqlc.SelectOutboxQueueRow) error {
	switch event.Type {
	// add each type as a switch case
	case sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT:
		return svc.handleAdvanceTournamentEvent(ctx, event.Data)
	default:
		return fmt.Errorf("unknown outbox queue event type %s", event.Type)
	}
}

func (svc *Services) pollOutboxQueueEvents(ctx context.Context, querier sqlc.Querier) error {
	return pollOutboxQueueEvents(ctx, querier, svc.handleOutboxEvent, svc.EntropySource.GetNow)
}

func pollOutboxQueueEvents(ctx context.Context, querier sqlc.Querier, handleOutboxEvent handleOutboxEvent, getProcessedOn func() time.Time) error {
	eventRows, err := querier.SelectOutboxQueue(ctx, OutboxPollCount) // locks events for the duration of the handler
	if err != nil {
		return fmt.Errorf("failed to select %d messages from outbox queue: %w", OutboxPollCount, err)
	}

	type eventResult struct {
		eventID int64
		err     error
	}

	var wg sync.WaitGroup
	processedEvents := make([]eventResult, len(eventRows))

	for i, event := range eventRows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := handleOutboxEvent(ctx, event)

			processedEvents[i] = eventResult{eventID: event.ID, err: err}
		}()
	}

	wg.Done()

	processedEventIDs := make([]int64, 0, len(processedEvents))
	var errProcessedEvents []eventResult

	for _, event := range processedEvents {
		if event.err == nil {
			processedEventIDs = append(processedEventIDs, event.eventID)
		} else {
			errProcessedEvents = append(errProcessedEvents, event)
		}
	}
	if len(errProcessedEvents) > 0 {
		slog.ErrorContext(ctx, "failed to handle outbox queue event", "events", errProcessedEvents)
	}

	if err := querier.UpdateOutboxQueueProcessedByID(ctx, sqlc.UpdateOutboxQueueProcessedByIDParams{
		Ids:           processedEventIDs,
		ProcessedTime: pgtype.Timestamptz{Time: getProcessedOn(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to acknolwedge outbox queue messages %+v: %w", processedEvents, err)
	}

	return nil
}

func PollOutboxQueueLoop(ctx context.Context, svc *Services, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	for {
		select {
		case <-ticker.C:
			err := svc.DB.ExecTx(ctx, db.Tx{
				// ReadCommitted is used as a basic 'Default` isolation level, the primary purpose of the transaction is atomicity
				// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction
				Isolation:  pgx.ReadCommitted,
				QueryFn:    svc.pollOutboxQueueEvents,
				RetryCount: 1,
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to poll outbox queue", "err", err)
			}
		case <-ctx.Done():
			slog.InfoContext(ctx, "exiting outbox queue polling loop")
			return
		}
	}
}
