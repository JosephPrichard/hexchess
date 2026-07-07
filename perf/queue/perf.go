package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type PerfTestConfig struct {
	DurationSecs    int
	EventsPerSecond int
}

type PostgresPerfTest struct {
	PerfTestConfig
	EventKind     string
	GenerateInput func() []byte
}

const (
	RedisEventQueueTable = "redis_queue_metadata"
	EventQueueTable      = "event_queue"
)

func RunPostgresPerfTest(dataSources *DataSources, perftest PostgresPerfTest) (GrafanaMetric, error) {
	ctx := dataSources.Context
	pgGroupID := pgtype.UUID{Bytes: uuid.New(), Valid: true}

	batchTicker := time.NewTicker(time.Second)
	totalEventCount := perftest.DurationSecs * perftest.EventsPerSecond

	for i := range totalEventCount {
		eventInput := perftest.GenerateInput()
		_, err := dataSources.pgPool.Exec(ctx,
			"INSERT INTO $1 (type, data, group_id) VALUES ($2, $3, $4);",
			EventQueueTable,
			perftest.EventKind,
			eventInput,
			pgGroupID)
		if err != nil {
			return GrafanaMetric{}, err
		}
		if i%perftest.EventsPerSecond == 0 {
			<-batchTicker.C
		}
	}

	if err := pollEvents(ctx, dataSources, EventQueueTable, pgGroupID, totalEventCount); err != nil {
		return GrafanaMetric{}, err
	}
	grafanaMetric, err := getEventMetric(ctx, dataSources, EventQueueTable, pgGroupID)
	if err != nil {
		return GrafanaMetric{}, err
	}

	return makeMetric(perftest.EventKind, grafanaMetric), nil
}

type RedisPerfTest struct {
	PerfTestConfig
	StreamName    string
	GenerateInput func() (string, string)
}

func RunRedisPerfTest(dataSources *DataSources, perftest RedisPerfTest) (GrafanaMetric, error) {
	ctx := dataSources.Context
	groupID := uuid.New()
	pgGroupID := pgtype.UUID{Bytes: groupID, Valid: true}

	batchTicker := time.NewTicker(time.Second)
	totalEventCount := perftest.DurationSecs * perftest.EventsPerSecond

	for i := range totalEventCount {
		inputKey, inputData := perftest.GenerateInput()
		partitionKey := inputKey[len(inputKey)-1]

		dataSources.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: fmt.Sprintf("%s:{%c}", perftest.StreamName, partitionKey),
			Values: map[string]any{
				"data":    inputData,
				"groupId": groupID,
			},
		})

		if i%perftest.EventsPerSecond == 0 {
			<-batchTicker.C
		}
	}

	if err := pollEvents(ctx, dataSources, RedisEventQueueTable, pgGroupID, totalEventCount); err != nil {
		return GrafanaMetric{}, err
	}
	grafanaMetric, err := getEventMetric(ctx, dataSources, RedisEventQueueTable, pgGroupID)
	if err != nil {
		return GrafanaMetric{}, err
	}

	return makeMetric(perftest.StreamName, grafanaMetric), nil
}

func durationToMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func pollEvents(ctx context.Context, dataSources *DataSources, tableName string, groupID pgtype.UUID, expectedTotalEventCount int) error {
	type countRow struct {
		Total int64 `db:"total"`
	}
	getCount := func() (countRow, error) {
		rows, err := dataSources.pgPool.Query(ctx, "SELECT COUNT(*) as total FROM $1 WHERE group_id = $2;",
			tableName,
			groupID)
		if err != nil {
			return countRow{}, err
		}
		defer rows.Close()
		return pgx.CollectOneRow(rows, pgx.RowToStructByName[countRow])
	}

	for range time.NewTicker(time.Second).C {
		count, err := getCount()
		if err != nil {
			return err
		}
		if count.Total == int64(expectedTotalEventCount) {
			break
		}
	}
	return nil
}

func getEventMetric(ctx context.Context, dataSources *DataSources, tableName string, groupID pgtype.UUID) (GrafanaTrendValues, error) {
	rows, err := dataSources.pgPool.Query(ctx, "SELECT id, consumed_on, processed_on FROM $1 WHERE group_id = $2;",
		tableName,
		groupID)
	if err != nil {
		return GrafanaTrendValues{}, err
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[struct {
		ID         int64              `db:"id"`
		ConsumedOn pgtype.Timestamptz `db:"consumed_on"`
		ProcesedOn pgtype.Timestamptz `db:"processed_on"`
	}])
	if err != nil {
		return GrafanaTrendValues{}, err
	}

	var eventLatencies []float64
	for _, event := range eventRows {
		if !event.ProcesedOn.Valid || !event.ConsumedOn.Valid {
			slog.Warn("selected event result record is missing measured outputs", "eventRows", eventRows)
		}
		eventLatencies = append(eventLatencies, durationToMillis(event.ProcesedOn.Time.Sub(event.ConsumedOn.Time)))
	}

	return BuildGrafanaTrend(eventLatencies)
}

func makeMetric(name string, trend GrafanaTrendValues) GrafanaMetric {
	return GrafanaMetric{
		Name:     fmt.Sprintf("jobqueue_perf_%s", name),
		Type:     "trend",
		Contains: "time",
		Values:   trend,
	}
}
