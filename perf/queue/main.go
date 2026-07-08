package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"hexchess-lib/config"
	"hexchess-lib/dotenv"
	"hexchess-lib/logutil"
	"log/slog"
	"os"
	"perf-test-queue/perf"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	ServiceName = "hexchess-queue-perftest"

	AdvanceTournamentEventName = "ADVANCE_TOURNAMENT_EVENT"
	FinishGameEventsName       = "finish_game_events"
	UpdtGameEventsName         = "updt_game_meta_events"

	SmokeProfile    = "SMOKE"
	CapacityProfile = "CAPACITY"
)

var PerfTestConfigs = map[string]map[string]perf.PerfTestConfig{
	SmokeProfile: {
		AdvanceTournamentEventName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 1,
		},
		FinishGameEventsName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 5,
		},
		UpdtGameEventsName: {
			Duration:        5 * time.Second,
			EventsPerSecond: 10,
		},
	},
	CapacityProfile: {
		AdvanceTournamentEventName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 10,
		},
		FinishGameEventsName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 50,
		},
		UpdtGameEventsName: {
			Duration:        1 * time.Minute,
			EventsPerSecond: 100,
		},
	},
}

var (
	testProfile = flag.String("testProfile", SmokeProfile, "test profile specifing the intensity of the workload")
	testOutfile = flag.String("testOutfile", "outfile.json", "file to dump output metrics to")
	timeoutSecs = flag.Int("testTimeoutSecs", 0, "max number of seconds for the text to run before cancellation")
	minUserID   = flag.Int("minUserID", 1, "minimum user ID to select")
	maxUserID   = flag.Int("maxUserID", 1000, "maximum user ID to select")
)

func main() {
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

	generator := perf.InputGenerator{MinUserID: int64(*minUserID), MaxUserID: int64(*maxUserID)}

	postgresPerftests := []perf.PostgresPerfTest{
		// {
		// 	PerfTestConfig: profileConfig[AdvanceTournamentEventName],
		// 	EventKind:      AdvanceTournamentEventName,
		// 	GenerateInput:  generateAdvanceTournamentInput,
		// },
	}

	redisPerftests := []perf.RedisPerfTest{
		{
			PerfTestConfig: profileConfig[FinishGameEventsName],
			StreamName:     FinishGameEventsName,
			GenerateInput:  generator.GenerateFinishGameInput,
		},
		{
			PerfTestConfig: profileConfig[UpdtGameEventsName],
			StreamName:     UpdtGameEventsName,
			GenerateInput:  generator.GenerateUpdtGameInput,
		},
	}

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
		Addrs:          rdbPrimaryNodes,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		MaxRedirects:   8,
		RouteRandomly:  false,
		RouteByLatency: false,
	})

	state := perf.State{Context: ctx, PGPool: pool, RedisClient: redisClient}

	var metrics []perf.GrafanaMetric
	var metricErrs error

	for _, perftest := range postgresPerftests {
		metricResult, err := perf.RunPostgresPerfTest(state, perftest)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}
	for _, perftest := range redisPerftests {
		metricResult, err := perf.RunRedisPerfTest(state, perftest)

		metrics = append(metrics, metricResult)
		metricErrs = errors.Join(metricErrs, err)
	}

	if metricErrs != nil {
		logutil.Fatal("collect all metrics", nil, "metricErrs", metricErrs)
	}

	summary := perf.GrafanaSummary{Metrics: make(map[string]perf.GrafanaMetric)}
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
		logutil.Fatal("failed to encode metrics to output file", err)
	}
}
