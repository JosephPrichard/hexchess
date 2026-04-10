package svc

import (
	"context"
	"hexchess-svc/db"
	"hexchess-svc/egress"
	"slices"
	"testing"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

type TestInfrastructure struct {
	DB    db.DB
	Redis db.Redis
	AWS   egress.AWS
}

type ServiceMocks struct {
	Entropy  EntropySource
	Remote   egress.RemoteAPIs
	S3Client egress.S3Client
}

func SetupServicesTest(t logutil.TestLogger, mocks ServiceMocks, flags ...itest.TestFlag) (*HexchessServices, TestInfrastructure) {
	setup := &Setup{}

	eg, egCtx := errgroup.WithContext(t.Context())

	roPostgres := slices.Contains(flags, itest.ROPostgres)
	rwPostgres := slices.Contains(flags, itest.RWPostgres)
	redis := slices.Contains(flags, itest.Redis)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			setup.DB, err = itest.SetupPostgresTest(egCtx, t, rwPostgres)
			return
		})
	}
	if redis {
		eg.Go(func() (err error) {
			setup.Redis, err = itest.SetupRedisTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to mocks test state: %v", err)
	}

	setup.AWS = egress.AWS{S3Endpoint: "http://localhost:4566", S3Client: mocks.S3Client}

	services := MakeHexchessServices(Setup{
		DB:     setup.DB,
		Redis:  setup.Redis,
		AWS:    setup.AWS,
		Remote: mocks.Remote,
	})
	if mocks.Entropy != nil {
		services.entropy = mocks.Entropy
	}

	testInfra := TestInfrastructure{
		DB:    setup.DB,
		Redis: setup.Redis,
		AWS:   setup.AWS,
	}

	return services, testInfra
}

func AssertRedisChessState(t *testing.T, services *HexchessServices, wantState *ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	if wantState == nil {
		return
	}
	actualState, err := services.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("failed to retrieve in redis chess state assert: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}
