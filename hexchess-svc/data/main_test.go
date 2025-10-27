package data

import (
	"hexchess-svc/lib"
	"testing"
)

func TestMain(m *testing.M) {
	defer TeardownTestInfra()
	lib.InitLoggers(nil)
	m.Run()
}
