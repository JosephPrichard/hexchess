package db

import (
	"context"
	"hexchess-svc/lib/logutil"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jackc/pgx/v5"
)

const pgTokenRefreshPeriod = 10 * time.Minute

type PostgresTokenRefresher struct {
	stsClient *sts.Client
	token     atomic.Pointer[string]
	ticker    *time.Ticker
}

func NewPgTokenRefresher(ctx context.Context, region string) *PostgresTokenRefresher {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logutil.Fatal("load aws config", err)
	}
	refresher := &PostgresTokenRefresher{
		stsClient: sts.NewFromConfig(awsCfg),
		ticker:    time.NewTicker(pgTokenRefreshPeriod),
	}
	refresher.acquireToken()
	go refresher.refreshToken()
	return refresher
}

func (refresh *PostgresTokenRefresher) acquireToken() {
	result, err := refresh.stsClient.GetSessionToken(context.Background(), &sts.GetSessionTokenInput{})
	if err != nil {
		slog.Error("failed to generate postgres auth token", "error", err)
		return
	}
	refresh.token.Store(result.Credentials.SecretAccessKey)
}

func (refresh *PostgresTokenRefresher) refreshToken() {
	for range refresh.ticker.C {
		refresh.acquireToken()
	}
}

func (refresh *PostgresTokenRefresher) BeforeConnectFunc(ctx context.Context, cfg *pgx.ConnConfig) error {
	token := refresh.token.Load()
	if token != nil {
		cfg.Password = *token
	} else {
		slog.WarnContext(ctx, "before connect: token not available")
	}
	return nil
}
