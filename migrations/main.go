package main

import (
	"context"
	"fmt"
	"log"
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

	dbURL := os.Getenv("DB_URL")
	awsRegion := os.Getenv("AWS_REGION")
	profile := os.Getenv("ACTIVE_PROFILE")

	// step 1: parse configuration
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("parse postgres config: %s", err)
	}

	// note(Joseph): password should NOT be here or else...
	slog.Info("parsed connection config", "connString", poolCfg.ConnConfig.ConnString())

	// step 2: override password with aws token
	if profile != "local" {
		slog.Info("retrieving RDS token")

		awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithRegion(awsRegion),
			awsConfig.WithRetryMaxAttempts(5),
			awsConfig.WithRetryMode(aws.RetryModeAdaptive),
		)
		if err != nil {
			log.Fatalf("load aws config: %s", err)
		}

		endpoint := fmt.Sprintf("%s:%d", poolCfg.ConnConfig.Host, poolCfg.ConnConfig.Port)
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, awsRegion, poolCfg.ConnConfig.User, awsCfg.Credentials)
		if err != nil {
			log.Fatalf("build postgres auth token: %s", err)
		}

		poolCfg.BeforeConnect = func(_ context.Context, cfg *pgx.ConnConfig) error {
			cfg.Password = token
			return nil
		}
	}

	// step 3: connect and execute migrations
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("create postgres db pool: %s", err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	startCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if _, err := db.ExecContext(startCtx, "SELECT 1;"); err != nil {
		log.Fatalf("execute startup query: %s", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set database dialect: %s", err)
	}

	if err := goose.Up(db, "."); err != nil {
		log.Fatalf("apply migration: %s", err)
	}

	slog.Info("migration: finished execution")

	// program stays running so the service does not restart when migration is complete
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("failed while serving", "error", err)
	}
}
