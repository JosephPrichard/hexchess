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

	var pool *pgxpool.Pool
	var redisAddr string
	var localstackAddr string

	eg, egCtx := errgroup.WithContext(ctx)

	roPostgres := slices.Contains(flags, ROPostgres)
	rwPostgres := slices.Contains(flags, RWPostgres)
	redis := slices.Contains(flags, Redis)
	localstack := slices.Contains(flags, AWS)

	if roPostgres || rwPostgres {
		eg.Go(func() (err error) {
			pool, err = SetupPostgresTest(egCtx, t)
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

	var pdb db.Database
	if rwPostgres {
		testTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			t.Fatalf("failed to begin test txn: %v", err)
		}
		pdb = db.MakeFakeDB(testTx)
	} else if roPostgres {
		pdb = db.MakeDB(pool)
	}
	var querier sqlc.Querier
	if pdb != nil {
		querier = pdb.Querier()
	}

	var rdb db.Redis
	if redis {
		rdb, _ = db.MakeRedis(ctx, db.RedisCfg{
			Names: testutil.MakeTestNames(db.DefaultRedisNames),
			Addrs: db.RedisAddrs{
				SorAddr:    []string{redisAddr},
				PubsubAddr: redisAddr,
			},
			Profile: "local",
		})
	}

	var aws cloud.AWSClient
	if localstack {
		aws = cloud.MakeAWSClients(ctx, cloud.AWSClientConfig{
			Names:       testutil.MakeTestNames(cloud.DefaultAWSNames),
			Profile:     "local",
			AWSRegion:   "us-east-1",
			AWSEndpoint: localstackAddr,
		})
	}

	return TestInfra{Querier: querier, DB: pdb, Redis: rdb, AWS: aws}
}
