package tournament

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func leafError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

func pipelineExec(ctx context.Context, pipeline redis.Pipeliner) error {
	if _, err := pipeline.Exec(ctx); err != nil && !errors.Is(redis.Nil, err) {
		return fmt.Errorf("pipeline exec: %w", err)
	}
	return nil
}
