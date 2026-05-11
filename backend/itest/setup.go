package itest

import (
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/internal/logutil"
	"slices"
)

type TestInfra struct {
	DB    db.DB
	Redis db.Redis
}

func (i TestInfra) Close() {
	if i.DB != nil {
		i.DB.Close()
	}
	i.Redis.Close()
}

func SetupTestInfra(t logutil.TestLogger, flags ...TestFlag) TestInfra {
	var infra TestInfra

	eg, egCtx := errgroup.WithContext(t.Context())

	roPostgres := slices.Contains(flags, ROPostgres)
	rwPostgres := slices.Contains(flags, RWPostgres)
	redis := slices.Contains(flags, Redis)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			infra.DB, err = SetupPostgresTest(egCtx, t, rwPostgres)
			return
		})
	}
	if redis {
		eg.Go(func() (err error) {
			infra.Redis, err = SetupRedisTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to mocks test state: %v", err)
	}

	return infra
}
