package itest

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/testutil"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type TestInfra struct {
	Querier sqlc.Querier
	DB      db.Database
	Redis   db.Redis
	AWS     cloud.AWSClient
}

func (i TestInfra) Close() {
	i.Redis.Close()
	if i.DB != nil {
		i.DB.Close()
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

	var testinfra TestInfra

	if isRwPostgresFlag {
		testTx, err := pgPool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatalf("failed to begin test txn: %v", err)
		}
		testinfra.DB = db.NewFakeDB(testTx)
		testinfra.Querier = testinfra.DB.Querier()
	} else if isRoPostgresFlag {
		testinfra.DB = db.NewPostgresDBFromPool(pgPool)
		testinfra.Querier = testinfra.DB.Querier()
	}

	if isRedisFlag {
		testinfra.Redis = db.NewRedis(ctx, db.RedisConfig{
			Names:         testutil.NewTestNames(db.DefaultRedisNames),
			PrimaryAddr:   []string{redisAddr},
			PubsubAddr:    redisAddr,
			ActiveProfile: config.Local,
		})
	}

	if isLocalstackFlag {
		testinfra.AWS = cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
			AWSEndpoint:   localstackAddr,
			AWSRegion:     "us-east-1",
			Names:         testutil.NewTestNames(cloud.DefaultAWSNames),
			ActiveProfile: config.Local,
			AWSUsername:   "testing",
			AWSPassword:   "testing",
		})
	}

	return testinfra
}
