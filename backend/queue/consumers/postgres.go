package consumers

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/errutil"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/lib/serrors"
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

	kind         sqlc.QueueTypeEnum
	pollInterval time.Duration
	pollCount    int32
	maxEvents    uint64
	fn           ConsumeFunc
}

func (c *PostgresConsumer) Consume() error {
	ticker := time.NewTicker(c.pollInterval)
	i := uint64(0)
	for range ticker.C {
		if i >= c.maxEvents && c.maxEvents != 0 {
			break
		}
		i++

		err := c.poll()
		if err != nil {
			slog.ErrorContext(c.ctx, "failed to poll postgres queue", "error", err)
		}
		if errors.Is(err, context.Canceled) {
			break
		}
	}

	slog.InfoContext(c.ctx, "finished postgres queue consumer")
	return nil
}

func (c *PostgresConsumer) poll() error {
	return c.pdb.ExecTx(c.ctx, db.TxArgs{
		// ReadCommitted is used as a basic 'Default` isolation level, the primary purpose of the transaction is atomicity
		// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries *per event*
		Isolation:  pgx.ReadCommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			ctx = context.WithValue(ctx, logutil.Trace, uuid.NewString())
			//slog.InfoContext(ctx, "polling postgres queue for events", "kind", c.kind)

			// locks events for the duration of the function
			eventRows, err := querier.SelectQueueByPolling(ctx, sqlc.SelectQueueByPollingParams{
				Type:  c.kind,
				Limit: c.pollCount,
			})
			if err != nil {
				return serrors.Wrap("select postgres queue messages", err, "kind", c.kind, "limit", c.pollCount)
			}
			if len(eventRows) == 0 {
				return nil
			}

			slog.InfoContext(ctx, "handling postgres queue events", "events", eventRows)

			type eventResult struct {
				eventID int64
				err     error
			}

			var wg sync.WaitGroup
			processedEvents := make([]eventResult, len(eventRows))

			for i, event := range eventRows {
				wg.Go(func() {
					err := c.fn(ctx, event.Data)
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
			slog.Log(ctx, level, "handled postgres queue events", "errProcessedEvents", errProcessedEvents, "eventsIDsToAck", eventIDsToAck)

			if len(eventIDsToAck) > 0 {
				if err := querier.UpdateQueueProcessedByID(ctx, sqlc.UpdateQueueProcessedByIDParams{
					Ids:           eventIDsToAck,
					ProcessedTime: pgtype.Timestamptz{Time: c.entropy.GetTime(), Valid: true},
				}); err != nil {
					return serrors.Wrap("acknowledge postgres queue messages", err, "events", processedEvents)
				}
			}
			return nil
		},
	})
}
