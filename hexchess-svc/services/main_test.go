package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
)

func TestMain(m *testing.M) {
	defer db.TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
