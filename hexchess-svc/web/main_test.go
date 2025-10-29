package web

import (
	"hexchess-svc/data"
	"hexchess-svc/util"
	"testing"
)

func TestMain(m *testing.M) {
	defer data.TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
