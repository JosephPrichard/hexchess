package data

import (
	"hexchess-svc/lib"
	"testing"
)

func TestMain(m *testing.M) {
	defer TeardownTestInfra()
	lib.InitLogger(nil)
	m.Run()
}
