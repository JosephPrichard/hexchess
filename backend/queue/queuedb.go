package queue

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/internal/errutil"
	svc "hexchess-svc/service"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBQueueHandler struct {
	kind         sqlc.OutboxQueueTypeEnum
	pollInterval time.Duration
	pollCount    int32
	fn           func(ctx context.Context, bytes []byte) error
}

func StartDBQueueConsumers(ctx context.Context, services svc.HexchessAPI, pdb db.DB) {
	eventHandler := EventHandler{Services: services}

	handlerList := []DBQueueHandler{
		{
			kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
			pollInterval: 5 * time.Second,
			pollCount:    64,
			fn:           eventHandler.HandleCreateTournamentMatchesEvent,
		},
		{
			kind:         sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
			pollInterval: 1 * time.Second,
			pollCount:    32,
			fn:           eventHandler.HandleAdvanceTournamentEvent,
		},
	}

	handlerTable := make(map[sqlc.OutboxQueueTypeEnum]DBQueueHandler)
	for _, handler := range handlerList {
		handlerTable[handler.kind] = handler
	}

	queue := DBQueue{pdb: pdb, entropy: &svc.RealEntropySource{}}

	for kind, handler := range handlerTable {
		go queue.PollOutboxQueueLoop(ctx, handler)
		slog.InfoContext(ctx, "started postgres queue consumer for handler", "kind", kind)
	}
}

type DBQueue struct {
	pdb     db.DB
	entropy svc.EntropyAPI
}

func (q *DBQueue) PollOutboxQueueLoop(ctx context.Context, handler DBQueueHandler) {
	ticker := time.NewTicker(handler.pollInterval)
	for range ticker.C {
		err := q.PollOutboxQueueEventsTx(ctx, handler)
		if err != nil {
			slog.ErrorContext(ctx, "failed to poll postgres queue", "err", err)
		}
		if errors.Is(err, context.Canceled) {
			break
		}
	}
}

func (q *DBQueue) PollOutboxQueueEventsTx(ctx context.Context, handler DBQueueHandler) error {
	return q.pdb.ExecTx(ctx, db.TxArgs{
		// ReadCommitted is used as a basic 'Default` isolation level, the primary purpose of the transaction is atomicity
		// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries
		Isolation:  pgx.ReadCommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			//slog.InfoContext(ctx, "polling postgres queue for events", "kind", handler.kind)

			// locks events for the duration of the function
			eventRows, err := querier.SelectOutboxQueueByPolling(ctx, sqlc.SelectOutboxQueueByPollingParams{
				Type:  handler.kind,
				Limit: handler.pollCount,
			})
			if err != nil {
				return fmt.Errorf("failed to select %d messages for event kind %s from postgres queue: %w", handler.pollCount, handler.kind, err)
			}
			if len(eventRows) == 0 {
				return nil
			}

			type eventResult struct {
				eventID int64
				err     error
			}

			var wg sync.WaitGroup
			processedEvents := make([]eventResult, len(eventRows))

			for i, event := range eventRows {
				wg.Go(func() {
					err := handler.fn(ctx, event.Data)
					processedEvents[i] = eventResult{eventID: event.ID, err: err}
				})
			}

			wg.Wait()

			eventIDsToAck := make([]int64, 0, len(processedEvents))
			var errProcessedEvents []eventResult

			for _, event := range processedEvents {
				// acknowledge the event if there is no error or the error is not retryable
				if event.err == nil || errutil.IsType[NonRetryableQueueError](event.err) {
					eventIDsToAck = append(eventIDsToAck, event.eventID)
				}
				// always collect all errors to be logged
				if event.err != nil {
					errProcessedEvents = append(errProcessedEvents, event)
				}
			}

			level := slog.LevelInfo
			if len(errProcessedEvents) > 0 {
				level = slog.LevelError
			}
			slog.Log(ctx, level, "handling postgres queue events", "errProcessedEvents", errProcessedEvents, "eventsIDsToAck", eventIDsToAck)

			if err := querier.UpdateOutboxQueueProcessedByID(ctx, sqlc.UpdateOutboxQueueProcessedByIDParams{
				Ids:           eventIDsToAck,
				ProcessedTime: pgtype.Timestamptz{Time: q.entropy.GetTime(), Valid: true},
			}); err != nil {
				return fmt.Errorf("failed to acknolwedge postgres queue messages %+v: %w", processedEvents, err)
			}

			return nil
		},
	})
}
