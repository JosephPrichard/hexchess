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

type authToken struct {
	value    string
	issuedAt time.Time
}

type PGConnector struct {
	stsClient *sts.Client
	token     atomic.Value
	ticker    *time.Ticker
}

func NewPgConnector(ctx context.Context, region string) *PGConnector {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		logutil.Fatal("load aws config", err)
	}
	connector := &PGConnector{
		stsClient: sts.NewFromConfig(awsCfg),
		ticker:    time.NewTicker(pgTokenRefreshPeriod),
	}
	go connector.refreshLoop()
	return connector
}

func (c *PGConnector) Stop() {
	c.ticker.Stop()
}

func (c *PGConnector) refreshLoop() {
	for range c.ticker.C {
		result, err := c.stsClient.GetSessionToken(context.Background(), &sts.GetSessionTokenInput{})
		if err != nil {
			slog.Error("failed to generate aws token", "error", err)
			continue
		}
		c.token.Store(authToken{value: *result.Credentials.SecretAccessKey, issuedAt: time.Now()})
	}
}

func (c *PGConnector) BeforeConnect(ctx context.Context, cfg *pgx.ConnConfig) error {
	token := c.token.Load().(authToken)
	cfg.Password = token.value
	slog.InfoContext(ctx, "before postgres connection", "tokenIssuedAt", token.issuedAt, "tokenLifetime", time.Since(token.issuedAt))
	return nil
}
