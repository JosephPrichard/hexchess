package svc

import (
	"errors"
	"testing"
)

func TestDynLog(t *testing.T) {
	dynLog("hello world", errors.New("test"), "arg1", 0, "arg2", "value")
	dynLog("hello world", nil, "arg1", 0, "arg2", "value")

	// check for output in logs to contain
	// ERROR hello world err=test arg1=0 arg2=value
	// INFO hello world arg1=0 arg2=value
}
