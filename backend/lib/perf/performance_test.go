package perf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPerformance_New(t *testing.T) {
	perf := WithContext(t.Context())

	assert.Equal(t, "hexchess-svc/lib/perf.TestPerformance_New", perf.name)
}
