package database

import (
	"context"
	"fmt"
	"hexchess-svc/utils/slogutil"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	rdsAuth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
)

func PostgresBeforeConnect(ctx context.Context, connCfg *pgx.ConnConfig, awsRegion string) func(ctx context.Context, cfg *pgx.ConnConfig) error {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		slogutil.Fatal("load aws config", err)
	}

	host := connCfg.Host
	port := connCfg.Port
	user := connCfg.User
	endpoint := fmt.Sprintf("%s:%d", host, port)

	return func(ctx context.Context, cfg *pgx.ConnConfig) error {
		slog.Info("building an RDS auth token", "endpoint", endpoint, "awsRegion", awsRegion, "user", user)

		// does not make a network call so this is safe to do before *each* connection
		token, err := rdsAuth.BuildAuthToken(ctx, endpoint, awsRegion, user, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build postgres auth token: %w", err)
		}
		cfg.Password = token
		return nil
	}
}
