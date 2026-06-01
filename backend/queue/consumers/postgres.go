package consumers

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/logutil"
	svc "hexchess-svc/service"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PostgresConsumer struct {
	ctx     context.Context
	pdb     db.DB
	entropy svc.EntropyAPI

	kind         sqlc.OutboxQueueTypeEnum
	pollInterval time.Duration
	pollCount    int32
	maxEvents    uint64
	fn           ConsumeFunc
}

func (q *PostgresConsumer) Consume() error {
	ticker := time.NewTicker(q.pollInterval)
	i := uint64(0)
	for range ticker.C {
		if i >= q.maxEvents && q.maxEvents != 0 {
			break
		}
		i++

		err := q.PollOutboxQueueEvents()
		if err != nil {
			slog.ErrorContext(q.ctx, "failed to poll postgres queue", "error", err)
		}
		if errors.Is(err, context.Canceled) {
			break
		}
	}

	slog.InfoContext(q.ctx, "finished postgres queue consumer")
	return nil
}

func (q *PostgresConsumer) PollOutboxQueueEvents() error {
	return q.pdb.ExecTx(q.ctx, db.TxArgs{
		// ReadCommitted is used as a basic 'Default` isolation level, the primary purpose of the transaction is atomicity
		// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries
		Isolation:  pgx.ReadCommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			ctx = context.WithValue(ctx, logutil.Trace, uuid.NewString())
			slog.InfoContext(ctx, "polling postgres queue for events", "kind", q.kind)

			// locks events for the duration of the function
			eventRows, err := querier.SelectOutboxQueueByPolling(ctx, sqlc.SelectOutboxQueueByPollingParams{
				Type:  q.kind,
				Limit: q.pollCount,
			})
			if err != nil {
				return fmt.Errorf("failed to select %d messages for event kind %s from postgres queue: %w", q.pollCount, q.kind, err)
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
					err := q.fn(ctx, event.Data)
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
