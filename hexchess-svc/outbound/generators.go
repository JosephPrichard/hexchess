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

type RandomGenerator struct{}

func (_ *RandomGenerator) MakeID() string {
	return uuid.NewString()
}

func (_ *RandomGenerator) GetNow() time.Time {
	return time.Now()
}

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
