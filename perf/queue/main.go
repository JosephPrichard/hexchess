package main

import (
	"context"
	"hexchess-lib/config"
	"hexchess-lib/dotenv"
	"hexchess-lib/logutil"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ServiceName = "hexchess-queue-perftest"
)

type DataSourceManager struct {
	ctx context.Context
	postgresDsn string
	redisAddrs   []string

	pool *pgxpool.Pool
	client redis.UniversalClient
}

func (manager *DataSourceManager) GetPostgres() *pgxpool.Pool {
	if manager.pool == nil {
		poolCfg, err := pgxpool.ParseConfig(manager.postgresDsn)
		if err != nil {
			logutil.Fatal("parse postgres config", err)
		}
		pool, err := pgxpool.NewWithConfig(manager.ctx, poolCfg)
		if err != nil {
			logutil.Fatal("create postgres pool", err)
		}
		manager.pool = pool
	}
	return manager.pool
}

func (manager *DataSourceManager) GetRedis() redis.UniversalClient {
	if manager.client == nil {
		manager.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:          manager.redisAddrs,
			DialTimeout:    5 * time.Second,
			ReadTimeout:    3 * time.Second,
			WriteTimeout:   3 * time.Second,
			MaxRedirects:   8,
			RouteRandomly:  false,
			RouteByLatency: false,
		})
	}
	return manager.client
}

type PostgresPerfTest struct {

}

func RunPostgresPerfTest(t PostgresPerfTest) {

}

type RedisPerfTest struct {
	
}

func RunRedisPerfTest(t RedisPerfTest) {

}

func main() {
	dotenv.Load()
	
	dbURL := os.Getenv("DB_URL")
	profile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	// awsRegion := os.Getenv("AWS_REGION")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	// rdbSorUsername := os.Getenv("REDIS_SOR_USERNAME")
	// rdbSorClusterName := os.Getenv("REDIS_SOR_CLUSTER_NAME")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	manager := &DataSourceManager{ctx: context.Background(), postgresDsn: dbURL, redisAddrs: rdbSorNodes}


}