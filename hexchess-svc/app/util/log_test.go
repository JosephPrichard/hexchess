package util

import (
	"errors"
	"testing"
)

func TestDynLog(t *testing.T) {
	DynLog("hello world", errors.New("test"), "arg1", 0, "arg2", "value")
	DynLog("hello world", nil, "arg1", 0, "arg2", "value")

	// check for output in logs to contain
	// ERROR hello world err=test arg1=0 arg2=value
	// INFO hello world arg1=0 arg2=value
}
