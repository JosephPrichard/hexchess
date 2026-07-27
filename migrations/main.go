package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	rdsAuth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
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

	// note(Joseph): password should NOT be here or else...
	slog.Info("parsed connection config", "connString", poolCfg.ConnConfig.ConnString())

	// step 2: override password with aws token
	if m.Profile != "local" {
		slog.Info("retrieving RDS token")

		awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithRegion(m.AWSRegion),
			awsConfig.WithRetryMaxAttempts(5),
			awsConfig.WithRetryMode(aws.RetryModeAdaptive),
		)
		if err != nil {
			return fmt.Errorf("load aws config: %s", err)
		}

		endpoint := fmt.Sprintf("%s:%d", poolCfg.ConnConfig.Host, poolCfg.ConnConfig.Port)
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, m.AWSRegion, poolCfg.ConnConfig.User, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build postgres auth token: %s", err)
		}

		poolCfg.BeforeConnect = func(_ context.Context, cfg *pgx.ConnConfig) error {
			cfg.Password = token
			return nil
		}
	}

	// step 3: connect and execute migrations
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create postgres db pool: %s", err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
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
