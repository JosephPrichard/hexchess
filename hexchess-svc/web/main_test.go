package web

import (
	"hexchess-svc/data"
	"hexchess-svc/logs"
	"testing"
)

func TestMain(m *testing.M) {
	defer data.TeardownTestInfra()
	logs.InitLogger(nil)
	m.Run()
}
