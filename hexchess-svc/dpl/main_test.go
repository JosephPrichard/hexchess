package dpl

import (
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"testing"
)

func TestMain(m *testing.M) {
	defer infra.TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
