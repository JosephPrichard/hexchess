package data

import (
	"hexchess-svc/logs"
	"testing"
)

func TestMain(m *testing.M) {
	defer TeardownTestInfra()
	logs.InitLogger(nil)
	m.Run()
}
