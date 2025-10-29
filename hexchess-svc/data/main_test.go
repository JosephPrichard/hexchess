package data

import (
	"hexchess-svc/util"
	"testing"
)

func TestMain(m *testing.M) {
	defer TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
