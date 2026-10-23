package slogutil

import (
	"errors"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"testing"
)

func TestCustomLogger(t *testing.T) {
	err1 := serrors.New("doing thing 1", errors.New("leaf"), "key1", "value1")
	err2 := serrors.New("doing thing 2", err1, "key2", "value2")
	err3 := serrors.New("doing thing 3", err2, "key2", "value3")

	slog.Error("test message", "error", err3, "key10", 1, "key20", 2)
}
