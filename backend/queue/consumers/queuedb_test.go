package consumers

import (
	"context"
	"errors"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/itest"
	"hexchess-svc/lib/testutil"
	svc "hexchess-svc/service"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollOutboxQueueEvents(t *testing.T) {
	tests := []struct {
		name               string
		kind               sqlc.OutboxQueueTypeEnum
		inputEvents        []sqlc.InsertOutboxQueueParams
		makeProcessFn      func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error
		wantEvents         []sqlc.OutboxQueue
		wantCapturedEvents []string
	}{
		{
			name: "TestCreateMatchesEvent",
			kind: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
			inputEvents: []sqlc.InsertOutboxQueueParams{
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:      []byte("test2"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					*capturedEvents = append(*capturedEvents, string(bytes))
					return nil
				}
			},
			wantEvents: []sqlc.OutboxQueue{
				{
					Type:        sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:        []byte("test1"),
					CreatedOn:   pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
					ProcessedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:      []byte("test2"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
					// since pollCount was 1, expect to only process the first event
				},
			},
			wantCapturedEvents: []string{"test1"},
		},
		{
			name: "TestDoesNotProcessRetryableErrorEvents",
			kind: sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
			inputEvents: []sqlc.InsertOutboxQueueParams{
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					return errors.New("test error")
				}
			},
			wantEvents: []sqlc.OutboxQueue{
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTCREATEMATCHESEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
					// expect evens to be not acknowledged
				},
			},
			wantCapturedEvents: []string{},
		},
		{
			name: "TestProcessesRetryableErrorEvents",
			kind: sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
			inputEvents: []sqlc.InsertOutboxQueueParams{
				{
					Type:      sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:      []byte("test1"),
					CreatedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
			},
			makeProcessFn: func(capturedEvents *[]string) func(ctx context.Context, bytes []byte) error {
				return func(ctx context.Context, bytes []byte) error {
					*capturedEvents = append(*capturedEvents, string(bytes))
					return NonRetryableQueueError{Err: errors.New("test error")}
				}
			},
			wantEvents: []sqlc.OutboxQueue{
				{
					Type:        sqlc.OutboxQueueTypeEnumTOURNAMENTADVANCEEVENT,
					Data:        []byte("test1"),
					CreatedOn:   pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
					ProcessedOn: pgtype.Timestamptz{Time: itest.TimeNow, Valid: true},
				},
			},
			wantCapturedEvents: []string{"test1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testinfra := itest.SetupTestInfra(t, itest.RWPostgres)
			defer testinfra.Close()

			ctx := t.Context()

			for _, params := range tt.inputEvents {
				err := testinfra.Querier.InsertOutboxQueue(ctx, params)
				require.NoError(t, err)
			}

			entropy := &svc.StableEntropySource{CurrTime: itest.TimeNow}

			queue := DBQueue{pdb: testinfra.DB, entropy: entropy}

			capturedEvents := make([]string, 0)

			err := queue.PollOutboxQueueEventsTx(ctx, DBQueueHandler{
				kind:         tt.kind,
				pollInterval: time.Microsecond,
				pollCount:    1,
				fn:           tt.makeProcessFn(&capturedEvents),
			})
			require.NoError(t, err)

			assert.Equal(t, tt.wantCapturedEvents, capturedEvents)

			outboxEvents, err := testinfra.Querier.SelectALLOutboxQueue(ctx)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantEvents, outboxEvents, cmpopts.IgnoreFields(sqlc.OutboxQueue{}, "ID", "CreatedOn"))
		})
	}
}
