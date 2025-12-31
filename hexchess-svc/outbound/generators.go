package outbound

import (
	"github.com/google/uuid"
	"time"
)

// Generator is a generator for generating things my program determines "non-deterministic" and therefore must be mocked in tests
type Generator interface {
	MakeID() string
	GetNow() time.Time
}

// NDGenerator non-deterministic generator used in the production impl
type NDGenerator struct{}

func (_ *NDGenerator) MakeID() string {
	return uuid.NewString()
}

func (_ *NDGenerator) GetNow() time.Time {
	return time.Now()
}

// StableGenerator is a generator that returns predefined mock values
type StableGenerator struct {
	ID   string
	Time time.Time
}

func (g *StableGenerator) MakeID() string {
	return g.ID
}

func (g *StableGenerator) GetNow() time.Time {
	return g.Time
}
