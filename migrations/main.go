package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	rdsAuth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	slog.Info("begin migration app")

	ctx := context.Background()

	primaryDBUrl := os.Getenv("PRIMARY_DB_URL")
	metricsDBUrl := os.Getenv("METRICS_DB_URL")
	awsRegion := os.Getenv("AWS_REGION")
	profile := os.Getenv("ACTIVE_PROFILE")

	migrations := []Migration{
		{
			DBUrl:        primaryDBUrl,
			MigrationDir: "hexchess",
			Profile:      profile,
			AWSRegion:    awsRegion,
		},
		{
			DBUrl:        metricsDBUrl,
			MigrationDir: "metrics",
			Profile:      profile,
			AWSRegion:    awsRegion,
		},
	}

	for _, migration := range migrations {
		if err := runMigration(ctx, migration); err != nil {
			slog.Error("failed to apply migration", "error", err)
		}
	}

	// program stays running so the service does not restart when migration is complete
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("failed while serving", "error", err)
	}
}

type Migration struct {
	DBUrl        string
	MigrationDir string
	Profile      string
	AWSRegion    string
}

func runMigration(ctx context.Context, m Migration) error {
	slog.Info("running migration", "args", m)

	// step 1: parse configuration
	poolCfg, err := pgxpool.ParseConfig(m.DBUrl)
	if err != nil {
		return fmt.Errorf("parse postgres config: %s", err)
	}
	connCfg := poolCfg.ConnConfig

	// note(Joseph): password should NOT be here or else...
	slog.Info("parsed connection config", "connString", connCfg.ConnString())

	// step 2: override password with aws token
	if m.Profile != "local" {
		slog.Info("retrieving RDS token for DB password", "connString", connCfg.ConnString())

		awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithRegion(m.AWSRegion),
			awsConfig.WithRetryMaxAttempts(5),
			awsConfig.WithRetryMode(aws.RetryModeAdaptive),
		)
		if err != nil {
			return fmt.Errorf("load aws config: %s", err)
		}

		endpoint := fmt.Sprintf("%s:%d", connCfg.Host, connCfg.Port)
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, m.AWSRegion, connCfg.User, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build postgres auth token: %s", err)
		}

		slog.Info("acquired RDS token for DB password", "token", token)
		connCfg.Password = token
	}

	// step 3: connect and execute migrations
	connString := connCfg.ConnString()

	// note(Joseph): token will be here as password, care about logging it
	slog.Info("migration: connecting to database", "connString", connString)

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("connect to database: %s", err)
	}
	defer db.Close()

	startCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if _, err := db.ExecContext(startCtx, "SELECT 1;"); err != nil {
		return fmt.Errorf("execute startup query: %s", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set database dialect: %s", err)
	}
	if err := goose.Up(db, m.MigrationDir); err != nil {
		return fmt.Errorf("apply migration for migrationDir=%s: %s", m.MigrationDir, err)
	}

	slog.Info("migration: finished execution")
	return err
}
