package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"testing"
)

func TestMain(m *testing.M) {
	defer db.TeardownTestInfra()
	logutil.InitLoggers(nil)
	m.Run()
}
