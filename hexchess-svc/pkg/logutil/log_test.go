package logutil

import (
	"context"
	"errors"
	"testing"
)

func TestDynLog(t *testing.T) {
	t.Parallel()

	DynLog(context.Background(), "hello world", errors.New("testing"), "arg1", 0, "arg2", "value")
	DynLog(context.Background(), "hello world", nil, "arg1", 0, "arg2", "value")

	// check for output in lib to contain
	// ERROR hello world arg1=0 arg2=value err=testing
	// INFO hello world arg1=0 arg2=value
}
