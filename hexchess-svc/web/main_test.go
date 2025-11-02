package web

import (
	"flag"
	"hexchess-svc/data"
	"hexchess-svc/util"
	"strconv"
	"testing"
)

var defaultSseCount = 100
var sseCount = flag.String("sse-count", strconv.Itoa(defaultSseCount), "The number of SSE connections to be used in throughput tests.")

func sseCountFlag() int {
	if sseCount == nil {
		return defaultSseCount
	}
	c, err := strconv.Atoi(*sseCount)
	if err != nil {
		panic(err)
	}
	return c
}

func TestMain(m *testing.M) {
	defer flag.Parse()
	defer data.TeardownTestInfra()
	util.InitLoggers(nil)
	m.Run()
}
