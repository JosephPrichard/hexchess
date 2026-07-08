package perf

import (
	"encoding/json"
	"fmt"
	"hexchess-lib/logutil"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type PerfTestConfig struct {
	Duration        time.Duration `json:"duration"`
	EventsPerSecond int           `json:"eventsPerSecond"`
}

func (c PerfTestConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Duration        string `json:"duration"`
		EventsPerSecond int    `json:"eventsPerSecond"`
	}{
		Duration:        c.Duration.String(),
		EventsPerSecond: c.EventsPerSecond,
	})
}

type PostgresPerfTest struct {
	PerfTestConfig
	EventKind     string        `json:"eventKind"`
	GenerateInput func() []byte `json:"-"`
}

const (
	EventQueueTable      = "event_queue"
	RedisEventQueueTable = "redis_queue_metrics"
)

func RunPostgresPerfTest(state State, perftest PostgresPerfTest) (GrafanaMetric, error) {
	groupID := uuid.New()

	state.WithValue(logutil.GroupID, groupID)

	batchTicker := time.NewTicker(time.Second)
	totalEventCount := int(perftest.Duration.Seconds()) * perftest.EventsPerSecond

	slog.Info("insert events into postgres event queue", "perftest", perftest)

	for i := range totalEventCount {
		inputData := perftest.GenerateInput()

		_, err := state.PGPool.Exec(state.Context,
			fmt.Sprintf("INSERT INTO %s (type, data, group_id) VALUES ($1, $2, $3);", EventQueueTable),
			perftest.EventKind,
			inputData,
			pgtype.UUID{Bytes: groupID, Valid: true})
		if err != nil {
			return GrafanaMetric{}, fmt.Errorf("insert into event queue table: %w", err)
		}

		if i%perftest.EventsPerSecond == 0 {
			<-batchTicker.C
			slog.Info("iteration of insert events into event queue table", "index", i)
		}
	}

	if err := pollEvents(state, EventQueueTable, groupID, totalEventCount); err != nil {
		return GrafanaMetric{}, err
	}
	grafanaMetric, err := getEventMetric(state, EventQueueTable, groupID)
	if err != nil {
		return GrafanaMetric{}, err
	}

	return GrafanaMetric{
		Name:     fmt.Sprintf("jobqueue_perf_%s", perftest.EventKind),
		Type:     "trend",
		Contains: "time",
		Values:   grafanaMetric,
	}, nil
}

type RedisPerfTest struct {
	PerfTestConfig
	StreamName    string                                    `json:"streamName"`
	GenerateInput func() (partitionKey string, data []byte) `json:"-"`
}

func RunRedisPerfTest(state State, perftest RedisPerfTest) (GrafanaMetric, error) {
	groupID := uuid.New()

	state.WithValue(logutil.GroupID, groupID)

	batchTicker := time.NewTicker(time.Second)
	totalEventCount := int(perftest.Duration.Seconds()) * perftest.EventsPerSecond

	slog.Info("insert events into redis stream", "perftest", perftest, "totalEventCount", totalEventCount)

	for i := range totalEventCount {
		partitionKey, inputData := perftest.GenerateInput()

		xargs := &redis.XAddArgs{
			Stream: fmt.Sprintf("%s:{%s}", perftest.StreamName, partitionKey),
			Values: map[string]any{
				"data":    string(inputData),
				"groupId": groupID.String(),
			},
		}
		if err := state.RedisClient.XAdd(state.Context, xargs).Err(); err != nil {
			return GrafanaMetric{}, fmt.Errorf("")
		}

		if i%perftest.EventsPerSecond == 0 {
			<-batchTicker.C
			slog.Info("iteration of insert events into redis stream", "index", i)
		}
	}

	if err := pollEvents(state, RedisEventQueueTable, groupID, totalEventCount); err != nil {
		return GrafanaMetric{}, err
	}
	grafanaMetric, err := getEventMetric(state, RedisEventQueueTable, groupID)
	if err != nil {
		return GrafanaMetric{}, err
	}

	return GrafanaMetric{
		Name:     fmt.Sprintf("jobqueue_perf_%s", perftest.StreamName),
		Type:     "trend",
		Contains: "time",
		Values:   grafanaMetric,
	}, nil
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

func getEventMetric(state State, tableName string, groupID uuid.UUID) (GrafanaTrend, error) {
	slog.Info("getting event metrics", "tableName", tableName)

	rows, err := state.PGPool.Query(state.Context,
		fmt.Sprintf("SELECT consumed_on, processed_on FROM %s WHERE group_id = $1;", tableName),
		pgtype.UUID{Bytes: groupID, Valid: true})
	if err != nil {
		return GrafanaTrend{}, fmt.Errorf("select event metrics for table %s: %w", tableName, err)
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[struct {
		ConsumedOn pgtype.Timestamptz `db:"consumed_on"`
		ProcesedOn pgtype.Timestamptz `db:"processed_on"`
	}])
	if err != nil {
		return GrafanaTrend{}, err
	}

	var eventLatencies []float64
	for _, event := range eventRows {
		if !event.ProcesedOn.Valid || !event.ConsumedOn.Valid {
			slog.Warn("selected event result record is missing measured outputs", "eventRows", eventRows)
			continue
		}
		eventLatencies = append(eventLatencies, durationToMillis(event.ProcesedOn.Time.Sub(event.ConsumedOn.Time)))
	}

	slog.Info("retrieved event latencies", "tableName", tableName, "datapointCount", len(eventLatencies))

	return BuildGrafanaTrend(eventLatencies)
}

func durationToMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
