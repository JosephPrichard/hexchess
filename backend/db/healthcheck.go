package db

import (
	"context"
	"log/slog"

	"github.com/hellofresh/health-go/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewHealthcheck(pool *pgxpool.Pool) health.CheckFunc {
	return func(ctx context.Context) error {
		_, err := pool.Exec(ctx, "SELECT 1;")
		if err != nil {
			slog.ErrorContext(ctx, "healthcheck error", "error", err, "kind", "postgres")
		}
		return err
	}
}
