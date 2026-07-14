package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type State struct {
	Context     context.Context
	PGPool      *pgxpool.Pool
	RedisClient redis.UniversalClient
}

func (state *State) WithValue(key any, value any) {
	state.Context = context.WithValue(state.Context, key, value)
}

type TestConfig struct {
	Duration        time.Duration `json:"duration"`
	EventsPerSecond int           `json:"eventsPerSecond"`
}

func (c TestConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Duration        string `json:"duration"`
		EventsPerSecond int    `json:"eventsPerSecond"`
	}{
		Duration:        c.Duration.String(),
		EventsPerSecond: c.EventsPerSecond,
	})
}

type PostgresQuePerfTest struct {
	Config        TestConfig    `json:"config"`
	EventKind     string        `json:"eventKind"`
	GenerateInput func() []byte `json:"-"`
}

const (
	EventQueueTable      = "event_queue"
	RedisEventQueueTable = "redis_queue_metrics"
)

func RunPostgresQueTest(state State, perfTest PostgresQuePerfTest) (Metric, error) {
	groupID := uuid.New()

	state.Context = context.WithValue(state.Context, logutil.GroupID, groupID)

	totalEventCount, err := insertLoop(perfTest.Config, func() error {
		inputData := perfTest.GenerateInput()

		_, err := state.PGPool.Exec(state.Context,
			fmt.Sprintf("INSERT INTO %s (type, data, group_id) VALUES ($1, $2, $3);", EventQueueTable),
			perfTest.EventKind,
			inputData,
			pgtype.UUID{Bytes: groupID, Valid: true})

		return err
	})
	if err != nil {
		return Metric{}, fmt.Errorf("insert event queue table %s: %w", EventQueueTable, err)
	}

	if err := pollEvents(state, EventQueueTable, groupID, totalEventCount); err != nil {
		return Metric{}, err
	}
	grafanaMetric, err := getEventMetric(state, EventQueueTable, groupID)
	if err != nil {
		return Metric{}, err
	}

	return Metric{
		Name:     metricName(perfTest.EventKind),
		Type:     "trend",
		Contains: "time",
		Values:   grafanaMetric,
	}, nil
}

type RedisQuePerfTest struct {
	Config        TestConfig                                `json:"config"`
	StreamName    string                                    `json:"streamName"`
	GenerateInput func() (partitionKey string, data []byte) `json:"-"`
}

func RunRedisQueTest(state State, perfTest RedisQuePerfTest) (Metric, error) {
	groupID := uuid.New()

	state.Context = context.WithValue(state.Context, logutil.GroupID, groupID)

	totalEventCount, err := insertLoop(perfTest.Config, func() error {
		partitionKey, inputData := perfTest.GenerateInput()

		cmd := state.RedisClient.XAdd(state.Context, &redis.XAddArgs{
			Stream: fmt.Sprintf("%s:{%s}", perfTest.StreamName, partitionKey),
			Values: map[string]any{
				"data":    string(inputData),
				"groupId": groupID.String(),
			},
		})

		return cmd.Err()
	})
	if err != nil {
		return Metric{}, fmt.Errorf("insert event to redis stream %s: %w", perfTest.StreamName, err)
	}

	if err := pollEvents(state, RedisEventQueueTable, groupID, totalEventCount); err != nil {
		return Metric{}, err
	}
	grafanaMetric, err := getEventMetric(state, RedisEventQueueTable, groupID)
	if err != nil {
		return Metric{}, err
	}

	return Metric{
		Name:     metricName(perfTest.StreamName),
		Type:     "trend",
		Contains: "time",
		Values:   grafanaMetric,
	}, nil
}

func insertLoop(perfTest TestConfig, insert func() error) (int, error) {
	batchTicker := time.NewTicker(time.Second)
	totalEventCount := int(perfTest.Duration.Seconds()) * perfTest.EventsPerSecond

	slog.Info("begin insert event input loop", "perfTest", perfTest, "totalEventCount", totalEventCount)

	for i := range totalEventCount {
		if err := insert(); err != nil {
			return 0, err
		}
		if i%perfTest.EventsPerSecond == 0 {
			<-batchTicker.C
			slog.Info("iteration of insert event loop", "perfTest", perfTest, "index", i)
		}
	}

	return totalEventCount, nil
}

func pollEvents(state State, tableName string, groupID uuid.UUID, expectedTotalEventCount int) error {
	slog.Info("begin polling events", "tableName", tableName)

	type countRow struct {
		Total int64 `db:"total"`
	}
	getCount := func() (countRow, error) {
		rows, err := state.PGPool.Query(state.Context,
			fmt.Sprintf("SELECT COUNT(*) as total FROM %s WHERE group_id = $1;", tableName),
			pgtype.UUID{Bytes: groupID, Valid: true})
		if err != nil {
			return countRow{}, err
		}
		defer rows.Close()
		return pgx.CollectOneRow(rows, pgx.RowToStructByName[countRow])
	}

	i := 1
	for range time.NewTicker(time.Second).C {
		slog.Info("attempt polling events table", "table", tableName, "attempt", i)

		count, err := getCount()
		if err != nil {
			return fmt.Errorf("select event count for table %s: %w", tableName, err)
		}
		if count.Total == int64(expectedTotalEventCount) {
			break
		}
		i++
	}
	return nil
}

func getEventMetric(state State, tableName string, groupID uuid.UUID) (Trend, error) {
	slog.Info("getting event metrics", "tableName", tableName)

	rows, err := state.PGPool.Query(state.Context,
		fmt.Sprintf("SELECT consumed_on, processed_on FROM %s WHERE group_id = $1;", tableName),
		pgtype.UUID{Bytes: groupID, Valid: true})
	if err != nil {
		return Trend{}, fmt.Errorf("select event metrics for table %s: %w", tableName, err)
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[struct {
		ConsumedOn  pgtype.Timestamptz `db:"consumed_on"`
		ProcessedOn pgtype.Timestamptz `db:"processed_on"`
	}])
	if err != nil {
		return Trend{}, err
	}

	var eventLatencies []float64
	for _, event := range eventRows {
		if !event.ProcessedOn.Valid || !event.ConsumedOn.Valid {
			slog.Warn("selected event result record is missing measured outputs", "eventRows", eventRows)
			continue
		}
		duration := event.ProcessedOn.Time.Sub(event.ConsumedOn.Time)
		eventLatencies = append(eventLatencies, float64(duration)/float64(time.Millisecond))
	}

	slog.Info("retrieved event latencies", "tableName", tableName, "datapointCount", len(eventLatencies))

	return BuildGrafanaTrend(eventLatencies)
}
