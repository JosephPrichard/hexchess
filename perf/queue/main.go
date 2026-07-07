package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"hexchess-lib/config"
	"hexchess-lib/dotenv"
	"hexchess-lib/logutil"
	"hexchess-lib/parseutil"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ServiceName = "hexchess-queue-perftest"

	AdvanceTournamentEventName = "ADVANCE_TOURNAMENT_EVENT"

	SmokeProfile    = "SMOKE"
	CapacityProfile = "CAPACITY"
)

var (
	testProfile = flag.String("testProfile", SmokeProfile, "test profile specifing the intensity of the workload")
	testOutfile = flag.String("testOutfile", "outfile.json", "file to dump output metrics to")
)

type DataSources struct {
	Context     context.Context
	pgPool      *pgxpool.Pool
	redisClient redis.UniversalClient
}

var PerfTestConfigs = map[string]map[string]PerfTestConfig{
	SmokeProfile: {
		AdvanceTournamentEventName: {
			DurationSecs:    15,
			EventsPerSecond: 3,
		},
	},
	CapacityProfile: {
		AdvanceTournamentEventName: {
			DurationSecs:    int(time.Minute.Seconds() * 15),
			EventsPerSecond: 100,
		},
	},
}

func main() {
	dotenv.Load()

	dbURL := os.Getenv("DB_URL")
	runProfile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	// awsRegion := os.Getenv("AWS_REGION")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	// rdbSorUsername := os.Getenv("REDIS_SOR_USERNAME")
	// rdbSorClusterName := os.Getenv("REDIS_SOR_CLUSTER_NAME")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	timeoutSecs := parseutil.MustParseInt(os.Getenv("TEST_TIMEOUT"))

	profileConfig, ok := PerfTestConfigs[*testProfile]
	if !ok {
		logutil.Fatal("provided test profile is not valid", nil, "providedTestProfile", testProfile)
	}

	ctx := context.Background()
	cancel := func() {}
	if timeoutSecs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(timeoutSecs))
	}
	defer cancel()

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, runProfile)
	defer shutdown()

	postgresPerftests := []PostgresPerfTest{
		{
			PerfTestConfig: profileConfig[AdvanceTournamentEventName],
			EventKind:      AdvanceTournamentEventName,
		},
	}

	redisPerftests := []RedisPerfTest{}

	slog.Info("starting perf tests", "postgresPerftests", postgresPerftests, "redisPerftests", redisPerftests)

	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logutil.Fatal("failed to parse postgres DSN: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("failed to create pool: %v", err)
	}

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:          rdbSorNodes,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	})

	dataSources := &DataSources{Context: ctx, pgPool: pool, redisClient: redisClient}

	var metrics []GrafanaMetric
	var metricErrs error

	for _, perftest := range postgresPerftests {
		metricResult, err := RunPostgresPerfTest(dataSources, perftest)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}
	for _, perftest := range redisPerftests {
		metricResult, err := RunRedisPerfTest(dataSources, perftest)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}

	if metricErrs != nil {
		logutil.Fatal("failed to collect all metrics", nil, "metricErrs", metricErrs)
	}

	summary := GrafanaSummary{Metrics: make(map[string]GrafanaMetric)}
	for _, metric := range metrics {
		summary.Metrics[metric.Name] = metric
	}

	f, err := os.Create(*testOutfile)
	if err != nil {
		logutil.Fatal("failed to create output file", err, "file", testOutfile)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		logutil.Fatal("failed to encode metrics to output file", err)
	}
}
