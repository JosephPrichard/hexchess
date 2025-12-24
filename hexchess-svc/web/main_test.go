package web

import (
	"flag"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
)

func TestMain(m *testing.M) {
	defer flag.Parse()
	defer db.TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
