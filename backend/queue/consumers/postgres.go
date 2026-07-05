package consumers

import (
	"context"
	"errors"
	"hexchess-lib/errutil"
	"hexchess-lib/logutil"
	"hexchess-lib/serrors"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	svc "hexchess-svc/service"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PostgresConsumer struct {
	ctx     context.Context
	pdb     db.Database
	entropy svc.EntropyAPI

	EventKind    sqlc.QueueTypeEnum `json:"eventKind"`
	PollInterval time.Duration      `json:"pollInterval"`
	PollCount    int32              `json:"pollCount"`
	MaxEvents    uint64             `json:"maxEvents"`

	consumeFunc ConsumeFunc
}

func (c *PostgresConsumer) Consume() {
	if c.entropy == nil {
		c.entropy = &svc.RealEntropySource{}
	}

	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	i := uint64(0)
	for range ticker.C {
		if i >= c.MaxEvents && c.MaxEvents != 0 {
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
}

func (c *PostgresConsumer) poll() error {
	return c.pdb.ExecTx(c.ctx, db.TxArgs{
		// ReadCommitted is used as a basic `Default` isolation level, the primary purpose of the transaction is atomicity
		// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries *per event*
		Isolation:  pgx.ReadCommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			start := time.Now()
			ctx = context.WithValue(ctx, logutil.Trace, uuid.NewString())

			//slog.InfoContext(ctx, "polling postgres queue for events", "eventKind", c.EventKind)

			// locks events for the duration of the function
			eventRows, err := querier.SelectQueueByPolling(ctx, sqlc.SelectQueueByPollingParams{
				Type:  c.EventKind,
				Limit: c.PollCount,
			})
			if err != nil {
				return serrors.Wrap("select postgres queue messages", err, "eventKind", c.EventKind, "limit", c.PollCount)
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
					err := c.consumeFunc(ctx, event.Data)
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

			if len(eventIDsToAck) > 0 {
				if err := querier.UpdateQueueProcessedByID(ctx, sqlc.UpdateQueueProcessedByIDParams{
					Ids:           eventIDsToAck,
					ProcessedTime: pgtype.Timestamptz{Time: c.entropy.GetTime(), Valid: true},
				}); err != nil {
					return serrors.Wrap("acknowledge postgres queue messages", err, "events", processedEvents)
				}
			}

			slog.InfoContext(ctx, "handled postgres queue events", "eventKind", c.EventKind,
				"errProcessedEvents", errProcessedEvents, "eventsIDsToAck", eventIDsToAck, "timeTaken", time.Since(start))
			return nil
		},
	})
}
