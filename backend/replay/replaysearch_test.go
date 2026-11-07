package replay

import (
	"hexchess-svc/itest"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/slogutil"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupSearcherTest(t slogutil.TestLogger) (*ReplaySearchService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t)

	service := NewReplaySearchService(infra.Database)

	return service, infra
}

func TestSearchReplaysByQuery(t *testing.T) {
	tests := []struct {
		name        string
		replayQuery ReplaysQuery
		wantReplays []FullReplay
	}{
		{
			name: "QueryBy_Users",
			replayQuery: ReplaysQuery{
				UserID:  opt.Some(int64(1)),
				AfterID: opt.None[int64](),
				PerPage: 5,
			},
			wantReplays: []FullReplay{
				TestReplays[4],
				TestReplays[3],
				TestReplays[2],
				TestReplays[0],
			},
		},
		{
			name: "QueryBy_Users_Cursor",
			replayQuery: ReplaysQuery{
				UserID:  opt.Some(int64(1)),
				AfterID: opt.Some(int64(3)),
				PerPage: 5,
			},
			wantReplays: []FullReplay{TestReplays[0]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, testinfra := setupSearcherTest(t)
			defer testinfra.Close()

			ctx := t.Context()

			replayList, err := service.SearchReplaysByQuery(ctx, tt.replayQuery)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantReplays, replayList)
		})
	}
}
