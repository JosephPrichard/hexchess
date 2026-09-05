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
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	slog.Info("begin migration app")

	if err := run(); err != nil {
		slog.Error("failed to run migration", "err", err.Error())
	}

	slog.Info("finished executing migrations")

	// program stays running so the service does not restart when migration is complete
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("failed while serving", "error", err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	dbURL := os.Getenv("DB_URL")
	awsRegion := os.Getenv("AWS_REGION")
	profile := os.Getenv("ACTIVE_PROFILE")

	// parse configuration
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("parse postgres config: %s", err)
	}

	// note(Joseph): password should NOT be here or else...
	slog.Info("parsed connection config", "connString", poolCfg.ConnConfig.ConnString())

	// override password with aws token
	if profile != "" && profile != "local" {
		slog.Info("retrieving RDS token")

		awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithRegion(awsRegion),
			awsConfig.WithRetryMaxAttempts(5),
			awsConfig.WithRetryMode(aws.RetryModeAdaptive),
		)
		if err != nil {
			return fmt.Errorf("load aws config: %s", err)
		}

		endpoint := fmt.Sprintf("%s:%d", poolCfg.ConnConfig.Host, poolCfg.ConnConfig.Port)
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, awsRegion, poolCfg.ConnConfig.User, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build postgres auth token: %s", err)
		}

		poolCfg.BeforeConnect = func(_ context.Context, cfg *pgx.ConnConfig) error {
			cfg.Password = token
			return nil
		}
	}

	// connect and execute migrations
	slog.Info("begin applying migrations")

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create postgres database pool: %s", err)
	}
	defer pool.Close()

	riverMigrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %s", err)
	}
	if _, err := riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply river migration: %s", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set database dialect: %s", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply hexchess migration: %s", err)
	}

	return nil
}
