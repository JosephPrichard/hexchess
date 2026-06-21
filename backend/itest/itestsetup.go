package itest

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/lib/testutil"
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

	roPostgres := slices.Contains(flags, ROPostgres)
	rwPostgres := slices.Contains(flags, RWPostgres)
	redis := slices.Contains(flags, Redis)
	localstack := slices.Contains(flags, AWS)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			pgPool, err = SetupPostgresTest(egCtx, t)
			return
		})
	}
	if redis {
		eg.Go(func() (err error) {
			redisAddr, err = SetupRedisTest(egCtx, t)
			return
		})
	}
	if localstack {
		eg.Go(func() (err error) {
			localstackAddr, err = SetupLocalstackTest(egCtx, t)
			return
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("failed to setup test state: %v", err)
	}

	var testinfra TestInfra

	if rwPostgres {
		testTx, err := pgPool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatalf("failed to begin test txn: %v", err)
		}
		testinfra.DB = db.NewFakeDB(testTx)
		testinfra.Querier = testinfra.DB.Querier()
	} else if roPostgres {
		testinfra.DB = db.NewDB(pgPool)
		testinfra.Querier = testinfra.DB.Querier()
	}

	if redis {
		testinfra.Redis, _ = db.NewRedis(ctx, db.RedisCfg{
			Names: testutil.NewTestNames(db.DefaultRedisNames),
			Addrs: db.RedisAddrs{
				SorAddr:    []string{redisAddr},
				PubsubAddr: redisAddr,
			},
			ActiveProfile: "local",
		})
	}

	if localstack {
		testinfra.AWS = cloud.NewAWSClients(ctx, cloud.AWSClientConfig{
			Names:          testutil.NewTestNames(cloud.DefaultAWSNames),
			Profile:        "local",
			AWSRegion:      "us-east-1",
			AWSEndpoint:    localstackAddr,
			StaticUsername: "testing",
			StaticPassword: "testing",
		})
	}

	return testinfra
}
