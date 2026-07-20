package itest

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/metricsdb"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/testutil"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type TestInfra struct {
	PrimaryQuerier primarydb.Querier
	PrimaryDB      db.Database[primarydb.Querier]
	MetricsDB      db.Database[metricsdb.Querier]
	Redis          db.Redis
	AWS            cloud.AWSClient
}

func (i TestInfra) Close() {
	i.Redis.Close()
	if i.PrimaryDB != nil {
		i.PrimaryDB.Close()
	}
	if i.MetricsDB != nil {
		i.MetricsDB.Close()
	}
}

func SetupIntegrationTest(t logutil.TestLogger, flags ...TestFlag) TestInfra {
	ctx := t.Context()

	var pgPool *pgxpool.Pool
	var redisAddr string
	var localstackAddr string

	eg, egCtx := errgroup.WithContext(ctx)

	isRoPostgresFlag := slices.Contains(flags, ROPostgres)
	isRwPostgresFlag := slices.Contains(flags, RWPostgres)
	isRedisFlag := slices.Contains(flags, Redis)
	isLocalstackFlag := slices.Contains(flags, AWS)

	if isRoPostgresFlag || isRwPostgresFlag {
		eg.Go(func() (err error) {
			pgPool, err = SetupPostgresTest(egCtx, t)
			return
		})
	}
	if isRedisFlag {
		eg.Go(func() (err error) {
			redisAddr, err = SetupRedisTest(egCtx, t)
			return
		})
	}
	if isLocalstackFlag {
		eg.Go(func() (err error) {
			localstackAddr, err = SetupLocalstackTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to setup test infra: %v", err)
	}

	var infra TestInfra

	if isRwPostgresFlag {
		infra.PrimaryDB = db.NewFakePrimaryDB(t, pgPool)
		infra.MetricsDB = db.NewFakeMetricsDB(t, pgPool)

		infra.PrimaryQuerier = infra.PrimaryDB.Querier()
	} else if isRoPostgresFlag {
		infra.PrimaryDB = db.NewPrimaryDB(pgPool)
		infra.MetricsDB = db.NewMetricsDB(pgPool)

		infra.PrimaryQuerier = infra.PrimaryDB.Querier()
	}

	if isRedisFlag {
		infra.Redis = db.NewRedis(ctx, db.RedisConfig{
			Names:         testutil.NewTestNames(db.DefaultRedisNames),
			PrimaryAddr:   []string{redisAddr},
			PubsubAddr:    redisAddr,
			ActiveProfile: config.Local,
		})
	}

	if isLocalstackFlag {
		infra.AWS = cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
			AWSEndpoint:   localstackAddr,
			AWSRegion:     "us-east-1",
			Names:         testutil.NewTestNames(cloud.DefaultAWSNames),
			ActiveProfile: config.Local,
			AWSUsername:   "testing",
			AWSPassword:   "testing",
		})
	}

	return infra
}
