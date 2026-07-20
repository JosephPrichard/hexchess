package consumers

import (
	"context"
	"errors"
	"hexchess-svc/db"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PostgresConfig struct {
	// (required) event name for this specific consumer to handle
	EventKind primarydb.QueueTypeEnum `json:"eventKind"`
	// (required) time in between successive poll attempts. polling more frequently is worse for performance but improves responsiveness
	PollInterval time.Duration `json:"pollInterval"`
	// (required) max number of messages received per poll attempt. each event is handled on a separate goroutine
	PollCount int32 `json:"pollCount"`
	// (optional) change the behavior of the consumer for tests
	MaxEvents uint64 `json:"maxEvents"`
}

type PostgresConsumer struct {
	PostgresConfig
	// allows receiving cancellation signals
	ctx context.Context
	// connects to queue table in database to poll events. requires transaction management.
	primaryDB db.Database[primarydb.Querier]
	// an implementation for consuming a single event
	consumeFunc ConsumeFunc
}

func (consumer *PostgresConsumer) Consume() {
	ticker := time.NewTicker(consumer.PollInterval)
	defer ticker.Stop()

	i := uint64(0)
	for range ticker.C {
		if i >= consumer.MaxEvents && consumer.MaxEvents != 0 {
			break
		}
		i++

		err := consumer.poll()
		if err != nil {
			slog.ErrorContext(consumer.ctx, "failed to poll postgres queue", "error", err)
		}
		if errors.Is(err, context.Canceled) {
			break
		}
	}

	slog.InfoContext(consumer.ctx, "finished postgres queue consumer")
}

func (consumer *PostgresConsumer) poll() error {
	return consumer.primaryDB.ExecTx(consumer.ctx, db.TxArgs[primarydb.Querier]{
		// ReadCommitted is used as a basic `Default` isolation level, the primary purpose of the transaction is atomicity
		// if handlers fail to acknowledge an event, it is not marked as processed and the lock is released at the end of the transaction, to allow retries *per event*
		Isolation:  pgx.ReadCommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, querier primarydb.Querier) error {
			start := time.Now()
			ctx = context.WithValue(ctx, logutil.Trace, uuid.NewString())

			// locks events for the duration of the function
			eventRows, err := querier.SelectQueueByPolling(ctx, primarydb.SelectQueueByPollingParams{
				Type:  consumer.EventKind,
				Limit: consumer.PollCount,
			})
			if err != nil {
				return serrors.New("select postgres queue messages", err, "eventKind", consumer.EventKind, "limit", consumer.PollCount)
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
					err := consumer.consumeFunc(ctx, event.Data)
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
				if err := querier.UpdateQueueProcessedByID(ctx, primarydb.UpdateQueueProcessedByIDParams{
					Ids:           eventIDsToAck,
					ProcessedTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}); err != nil {
					return serrors.New("acknowledge postgres queue messages", err, "events", processedEvents)
				}
			}

			slog.InfoContext(ctx, "handled postgres queue events", "eventKind", consumer.EventKind,
				"errProcessedEvents", errProcessedEvents, "eventsIDsToAck", eventIDsToAck, "timeTaken", time.Since(start))
			return nil
		},
	})
}
