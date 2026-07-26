package db

import (
	"context"
	"fmt"
	"hexchess-svc/utils/logutil"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	rdsAuth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
)

func NewPgBeforeConnect(ctx context.Context, connCfg *pgx.ConnConfig, awsRegion string) func(ctx context.Context, cfg *pgx.ConnConfig) error {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsRegion),
		config.WithRetryMaxAttempts(5),
		config.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		logutil.Fatal("load aws config", err)
	}

	return func(ctx context.Context, cfg *pgx.ConnConfig) error {
		endpoint := fmt.Sprintf("%s:%d", connCfg.Host, connCfg.Port)

		slog.Info("building an rds auth token",
			"endpoint", endpoint, "awsRegion", awsRegion, "user", connCfg.User)

		// does not make a network call so this is safe to do before *each* connection
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, awsRegion, connCfg.User, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build postgres auth token: %w", err)
		}

		// IMPORTANT: keep this commented out unless debugging
		slog.Info("obtained an rds auth token", "token", token)

		cfg.Password = token
		return nil
	}
}
