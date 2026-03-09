package svc

import (
	"github.com/google/uuid"
	"time"
)

// EntropySource is a generator for generating things my program determines as "non-deterministic" and therefore must be mocked in tests
type EntropySource interface {
	MakeID() string
	GetNow() time.Time
}

// RealEntropySource non-deterministic entropy source that generates real data
type RealEntropySource struct{}

func (_ *RealEntropySource) MakeID() string {
	return uuid.NewString()
}

func (_ *RealEntropySource) GetNow() time.Time {
	return time.Now()
}

// StableEntropySource is an entropy source that returns predefined mock values
type StableEntropySource struct {
	ID   string
	Time time.Time
}

func (g *StableEntropySource) MakeID() string {
	return g.ID
}

func (g *StableEntropySource) GetNow() time.Time {
	return g.Time
}
