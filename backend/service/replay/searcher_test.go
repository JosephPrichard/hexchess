package replay

import (
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/testutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupSearcherTest(t alog.TestLogger, flags ...itest.TestFlag) (*ReplaySearchService, itest.TestInfra) {
	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewSearchService(infra.Database)

	return services, infra
}

func TestSearchReplaysByQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		replayQuery ReplaysQuery
		wantReplays []model.FullReplay
	}{
		{
			name: "QueryBy_Users",
			replayQuery: ReplaysQuery{
				UserID:  opt.Some(int64(1)),
				AfterID: opt.None[int64](),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{
				itest.TestReplays[4],
				itest.TestReplays[3],
				itest.TestReplays[2],
				itest.TestReplays[0],
			},
		},
		{
			name: "QueryBy_Users_Cursor",
			replayQuery: ReplaysQuery{
				UserID:  opt.Some(int64(1)),
				AfterID: opt.Some(int64(3)),
				PerPage: 5,
			},
			wantReplays: []model.FullReplay{itest.TestReplays[0]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, testinfra := setupSearcherTest(t, itest.ROPostgres)
			defer testinfra.Close()

			ctx := t.Context()

			replayList, err := services.SearchReplaysByQuery(ctx, tt.replayQuery)
			require.NoError(t, err)

			testutil.Equal(t, tt.wantReplays, replayList)
		})
	}
}
