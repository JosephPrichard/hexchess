package itest

import (
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/database"

	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/testutil"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type TestInfra struct {
	database.Database

	Redis cache.Redis

	AWS cloud.AWSClient
}

func (i TestInfra) Close() {
	i.Redis.Close()
	i.Database.Close()
}

func SetupIntegrationTest(t alog.TestLogger, flags ...TestFlag) TestInfra {
	ctx := t.Context()

	var dbPool *pgxpool.Pool
	var redisAddr string
	var localstackAddr string

	eg, egCtx := errgroup.WithContext(ctx)

	isRoPostgresFlag := slices.Contains(flags, ROPostgres)
	isRwPostgresFlag := slices.Contains(flags, RWPostgres)
	isRedisFlag := slices.Contains(flags, Redis)
	isLocalstackFlag := slices.Contains(flags, AWS)

	if isRoPostgresFlag || isRwPostgresFlag {
		eg.Go(func() (err error) {
			dbPool, err = SetupPostgresTest(egCtx, t)
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
		// rwPostgres flag substitutes a pool with a connection to enable parallel, independent tests
		infra.Database = database.NewFakeDatabase(t, dbPool)
	} else if isRoPostgresFlag {
		// roPostgres flag uses a real database pool to enable concurrent transactions
		infra.Database = database.NewDatabaseFromPool(dbPool)
	}

	if isRedisFlag {
		infra.Redis = cache.NewRedis(ctx, cache.RedisConfig{
			PrimaryAddr:   []string{redisAddr},
			PubsubAddr:    redisAddr,
			ActiveProfile: config.Local,

			// tests use independent key prefixes to enable parallel, independent tests
			Names: testutil.NewTestNames(cache.DefaultRedisNames),
		})
	}

	if isLocalstackFlag {
		infra.AWS = cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
			AWSEndpoint: localstackAddr,
			AWSRegion:   "us-east-1",
			AWSUsername: "testing",
			AWSPassword: "testing",

			// tests use independent buckets to enable parallel, independent tests
			Names:         *testutil.NewTestNames(cloud.DefaultAWSNames),
			ActiveProfile: config.Local,
		})
	}

	return infra
}
