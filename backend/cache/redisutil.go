package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func PipelineExec(ctx context.Context, pipeline redis.Pipeliner) error {
	if _, err := pipeline.Exec(ctx); err != nil && !errors.Is(redis.Nil, err) {
		return fmt.Errorf("pipeline exec: %w", err)
	}
	return nil
}
