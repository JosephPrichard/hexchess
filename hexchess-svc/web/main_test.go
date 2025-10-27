package web

import (
	"hexchess-svc/data"
	"hexchess-svc/lib"
	"testing"
)

func TestMain(m *testing.M) {
	defer data.TeardownTestInfra()
	lib.InitLoggers(nil)
	m.Run()
}
