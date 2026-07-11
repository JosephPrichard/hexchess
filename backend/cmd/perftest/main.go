package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"hexchess-svc/perftest"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/dotenv"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ServiceName = "hexchess-perftest"

	AdvanceTournamentQueName = "ADVANCE_TOURNAMENT_EVENT"
	FinishGameEventsQueName       = "finish_game_events"
	UpdtGameEventsQueName         = "updt_game_meta_events"

	SmokeProfile    = "SMOKE"
	CapacityProfile = "CAPACITY"
)

var PerfTestConfigs = map[string]map[string]perftest.PerfTestConfig{
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
	dotenv.Load()

	dbURL := os.Getenv("DB_URL")
	runProfile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	rdbPrimaryNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

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

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, runProfile)
	defer shutdown()

	// step 2: connect to backend infrastructure
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logutil.Fatal("parse postgres DSN: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logutil.Fatal("create pool: %v", err)
	}

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:          rdbPrimaryNodes,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	})

	state := perftest.State{Context: ctx, PGPool: pool, RedisClient: redisClient}

	// step 3: prepare precondition data, test inputs, and perf test configurations
	preconditionData, err := perftest.GetPreconditionData(state)
	if err != nil {
		logutil.Fatal("get dynamic inputs", err)
	}

	generator := perftest.InputGenerator{
		PreconditionData: preconditionData,
		StaticData: perftest.StaticData{
			MinUserID: int64(*minUserID),
			MaxUserID: int64(*maxUserID),
		},
	}

	postgresPerftests := []perftest.PostgresQuePerfTest{
		{
			Config:        profileConfig[AdvanceTournamentQueName],
			EventKind:     AdvanceTournamentQueName,
			GenerateInput: generator.GenerateAdvanceTournamentInput,
		},
	}
	redisPerftests := []perftest.RedisQuePerfTest{
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

	var metrics []perftest.Metric
	var metricErrs error

	for _, pt := range postgresPerftests {
		metricResult, err := perftest.RunPostgresQueTest(state, pt)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}
	for _, pt := range redisPerftests {
		metricResult, err := perftest.RunRedisQueTest(state, pt)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}

	if metricErrs != nil {
		logutil.Fatal("collect all metrics", nil, "metricErrs", metricErrs)
	}

	// step 5: publish results of perf tests in grafana format
	summary := perftest.Summary{Metrics: make(map[string]perftest.Metric)}
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
