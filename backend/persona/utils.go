package persona

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/constraints"
)

func average[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func pipelineExec(ctx context.Context, pipeline redis.Pipeliner) error {
	if _, err := pipeline.Exec(ctx); err != nil && !errors.Is(redis.Nil, err) {
		return fmt.Errorf("pipeline exec: %w", err)
	}
	return nil
}

func leafError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}
