package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"hexchess-svc/perf"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ServiceName = "hexchess-perftest"

	AdvanceTournamentQueName = "ADVANCE_TOURNAMENT_EVENT"
	FinishGameEventsQueName  = "finish_game_events"
	UpdtGameEventsQueName    = "updt_game_meta_events"

	SmokeProfile    = "SMOKE"
	CapacityProfile = "CAPACITY"
)

var PerfTestConfigs = map[string]map[string]perf.TestConfig{
	SmokeProfile: {
		AdvanceTournamentQueName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 1,
		},
		FinishGameEventsQueName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 5,
		},
		UpdtGameEventsQueName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 10,
		},
	},
	CapacityProfile: {
		AdvanceTournamentQueName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 10,
		},
		FinishGameEventsQueName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 50,
		},
		UpdtGameEventsQueName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 100,
		},
	},
}

var (
	testProfile = flag.String("testProfile", SmokeProfile, "test profile specifing the intensity of the workload")
	testOutfile = flag.String("testOutfile", "outfile.json", "file to dump output metrics to")

	timeoutSecs = flag.Int("testTimeoutSecs", 0, "max number of seconds for the text to run before cancellation")

	minUserID = flag.Int("minUserID", 1, "minimum user ID to select")
	maxUserID = flag.Int("maxUserID", 1000, "maximum user ID to select")
)

func main() {
	// step 1: parse CLI inputs for static input data
	cfg := config.Load()

	profileConfig, ok := PerfTestConfigs[*testProfile]
	if !ok {
		logutil.Fatal("provided test profile is not valid", nil, "providedTestProfile", testProfile)
	}

	ctx := context.Background()
	cancel := func() {}
	if *timeoutSecs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(*timeoutSecs))
	}
	defer cancel()

	shutdown := logutil.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	// step 2: connect to backend infrastructure
	primaryPool := connectDb(ctx, cfg.PrimaryDbURL)
	metricsPool := connectDb(ctx, cfg.MetricsDbURL)

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:          cfg.RedisPrimaryNodes,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	})

	state := perf.State{
		Context:     ctx,
		PrimaryPool: primaryPool,
		MetricsPool: metricsPool,
		RedisClient: redisClient,
	}

	// step 3: prepare precondition data, test inputs, and perf test configurations
	generator, err := perf.NewInputGenerator(state)
	if err != nil {
		logutil.Fatal("initialize input generator", err)
	}

	generator.MinUserID = int64(*minUserID)
	generator.MaxUserID = int64(*maxUserID)

	postgresPerftests := []perf.PostgresQuePerfTest{
		{
			Config:        profileConfig[AdvanceTournamentQueName],
			EventKind:     AdvanceTournamentQueName,
			GenerateInput: generator.GenerateAdvanceTournamentInput,
		},
	}
	redisPerftests := []perf.RedisQuePerfTest{
		{
			Config:        profileConfig[FinishGameEventsQueName],
			StreamName:    FinishGameEventsQueName,
			GenerateInput: generator.GenerateFinishGameInput,
		},
		{
			Config:        profileConfig[UpdtGameEventsQueName],
			StreamName:    UpdtGameEventsQueName,
			GenerateInput: generator.GenerateUpdtGameInput,
		},
	}

	// step 4: run performance tests
	slog.Info("starting perf tests", "postgresPerftests", postgresPerftests, "redisPerftests", redisPerftests)

	var metrics []perf.Metric
	var metricErrs error

	for _, pt := range postgresPerftests {
		metricResult, err := perf.RunPostgresQueTest(state, pt)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}
	for _, pt := range redisPerftests {
		metricResult, err := perf.RunRedisQueTest(state, pt)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}

	if metricErrs != nil {
		logutil.Fatal("collect all metrics", nil, "metricErrs", metricErrs)
	}

	// step 5: publish results of perf tests in Grafana format
	summary := perf.Summary{Metrics: make(map[string]perf.Metric)}
	for _, metric := range metrics {
		summary.Metrics[metric.Name] = metric
	}

	f, err := os.Create(*testOutfile)
	if err != nil {
		logutil.Fatal("create output file", err, "file", testOutfile)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		logutil.Fatal("encode metrics to output file", err)
	}
}

func connectDb(ctx context.Context, dbURL string) *pgxpool.Pool {
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logutil.Fatal("parse postgres DSN: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create pool: %v", err)
	}
	return pool
}
