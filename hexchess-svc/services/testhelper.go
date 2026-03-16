package svc

import (
	"context"
	"slices"
	"testing"

	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"
	"hexchess-svc/pkg/testutil"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

func SetupServicesTest(t logutil.TestLogger, flags ...itest.TestFlag) (services Services) {
	eg, egCtx := errgroup.WithContext(t.Context())

	roPostgres := slices.Contains(flags, itest.ROPostgres)
	rwPostgres := slices.Contains(flags, itest.RWPostgres)
	redis := slices.Contains(flags, itest.Redis)
	aws := slices.Contains(flags, itest.Aws)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			services.DB, err = itest.SetupPostgresTest(egCtx, t, rwPostgres)
			services.Queries = services.DB.Queries()
			return
		})
	}
	if redis {
		eg.Go(func() (err error) {
			services.Redis, err = itest.SetupRedisTest(egCtx, t)
			return
		})
	}
	if aws {
		eg.Go(func() (err error) {
			services.AWS, err = itest.SetupAwsTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to setup test state: %v", err)
	}

	services.EntropySource = &RealEntropySource{}
	return services
}

func AssertRedisChess(t *testing.T, s *Services, wantState ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	actualState, err := s.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("get chess for assert: %v", err)
	}
	testutil.Equal(t, wantState, *actualState, options...)
}

func AssertChessState(t *testing.T, wantState ChessState, actualState *ChessState, options ...cmp.Option) {
	t.Helper()
	if actualState == nil {
		t.Fatalf("chess state is nil")
	}
	testutil.Equal(t, wantState, *actualState, options...)
}
