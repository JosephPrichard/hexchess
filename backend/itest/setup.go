package itest

import (
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"
	"slices"
)

type TestInfra struct {
	Querier sqlc.Querier
	DB      db.DB
	Redis   db.Redis
}

func (i TestInfra) Close() {
	i.Redis.Close()
	if i.DB != nil {
		i.DB.Close()
	}
}

func SetupTestInfra(t logutil.TestLogger, flags ...TestFlag) TestInfra {
	var pdb db.DB
	var rdb db.Redis

	eg, egCtx := errgroup.WithContext(t.Context())

	roPostgres := slices.Contains(flags, ROPostgres)
	rwPostgres := slices.Contains(flags, RWPostgres)
	redis := slices.Contains(flags, Redis)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			pdb, err = SetupPostgresTest(egCtx, t, rwPostgres)
			return
		})
	}
	if redis {
		eg.Go(func() (err error) {
			rdb, err = SetupRedisTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to setup test state: %v", err)
	}

	var querier sqlc.Querier
	if pdb != nil {
		querier = pdb.Querier()
	}
	return TestInfra{Querier: querier, DB: pdb, Redis: rdb}
}
