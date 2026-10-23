package service

import (
	"context"
	"errors"
	"fmt"

	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/redis/go-redis/v9"
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
