package out

import (
	"github.com/google/uuid"
	"time"
)

// EntropySource is a generator for generating things my program determines "non-deterministic" and therefore must be mocked in tests
type EntropySource interface {
	MakeID() string
	GetNow() time.Time
}

// NDEntropySource non-deterministic entropy source used to generate data that cannot be predicted
type NDEntropySource struct{}

func (_ *NDEntropySource) MakeID() string {
	return uuid.NewString()
}

func (_ *NDEntropySource) GetNow() time.Time {
	return time.Now()
}

// StableSource is an entropy source that returns predefined mock values
type StableSource struct {
	ID   string
	Time time.Time
}

func (g *StableSource) MakeID() string {
	return g.ID
}

func (g *StableSource) GetNow() time.Time {
	return g.Time
}
