package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/utils/opt"
	"math"
	"time"

	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/constraints"
)

func keysOfObjects(objects []s3Types.Object) []string {
	var s []string
	for _, o := range objects {
		if o.Key == nil {
			continue
		}
		s = append(s, *o.Key)
	}
	return s
}

func keysOfObjectIDs(objects []s3Types.ObjectIdentifier) []string {
	var s []string
	for _, o := range objects {
		if o.Key == nil {
			continue
		}
		s = append(s, *o.Key)
	}
	return s
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

func average[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func toDayEpoch(optTime opt.Option[time.Time]) opt.Option[int32] {
	// uses the unix epoch in days for range queries on date. this truncates away timestamp precision regarding hours, seconds, etc.
	return opt.Option[int32]{Value: int32(optTime.Value.Unix() / 86400), Present: optTime.Present}
}

func probabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}
