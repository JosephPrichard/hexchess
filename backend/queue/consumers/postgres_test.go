package consumers

import (
	"context"
	"errors"
	"hexchess-lib/testutil"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresConsumer(t *testing.T) {
	tests := []struct {
		name               string
		kind               sqlc.QueueTypeEnum
		inputEvents        []sqlc.InsertQueueParams
		makeProcessFn      func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error
		wantEvents         []sqlc.EventQueue
		wantCapturedEvents []string
	}{
		{
			name: "TestHandlesEvent",
			kind: sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
			inputEvents: []sqlc.InsertQueueParams{
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
				},
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test2"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					*capturedEvents = append(*capturedEvents, string(bytes))
					return nil
				}
			},
			wantEvents: []sqlc.EventQueue{
				{
					Type:        sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:        []byte("test1"),
					CreatedOn:   pgtype.Timestamptz{Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: true},
				},
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test2"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
					// since pollCount was 1, expect to only process the first event
				},
			},
			wantCapturedEvents: []string{"test1"},
		},
		{
			name: "TestDoesNotProcessRetryableErrorEvents",
			kind: sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
			inputEvents: []sqlc.InsertQueueParams{
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					return errors.New("test error")
				}
			},
			wantEvents: []sqlc.EventQueue{
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
					// expect evens to be not acknowledged
				},
			},
			wantCapturedEvents: []string{},
		},
		{
			name: "TestProcessesRetryableErrorEvents",
			kind: sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
			inputEvents: []sqlc.InsertQueueParams{
				{
					Type:      sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					*capturedEvents = append(*capturedEvents, string(bytes))
					return NonRetryableQueueError{Err: errors.New("test error")}
				}
			},
			wantEvents: []sqlc.EventQueue{
				{
					Type:        sqlc.QueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:        []byte("test1"),
					CreatedOn:   pgtype.Timestamptz{Valid: true},
					ProcessedOn: pgtype.Timestamptz{Valid: true},
				},
			},
			wantCapturedEvents: []string{"test1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testinfra := itest.SetupIntegrationTest(t, itest.RWPostgres)
			defer testinfra.Close()

			ctx := t.Context()

			for _, params := range tt.inputEvents {
				err := testinfra.Querier.InsertQueue(ctx, params)
				require.NoError(t, err)
			}

			capturedEvents := make([]string, 0)

			queue := PostgresConsumer{
				ctx:         ctx,
				pdb:         testinfra.DB,
				consumeFunc: tt.makeProcessFn(&capturedEvents),
				PostgresConfig: PostgresConfig{
					EventKind:    tt.kind,
					PollInterval: time.Microsecond,
					PollCount:    1,
					MaxEvents:    1,
				},
			}

			queue.Consume()

			assert.Equal(t, tt.wantCapturedEvents, capturedEvents)

			outboxEvents, err := testinfra.Querier.SelectALLQueue(ctx)
			require.NoError(t, err)

			// ignores comparisons of timestamps by direct value, instead we check by nullability
			testutil.Equal(t, tt.wantEvents, outboxEvents, 
				cmpopts.IgnoreFields(sqlc.EventQueue{}, "ID"), 
				cmpopts.IgnoreFields(pgtype.Timestamptz{}, "Time"))
		})
	}
}
