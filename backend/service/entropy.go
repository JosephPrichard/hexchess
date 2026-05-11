package svc

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

// EntropySource is a generator for generating things my program determines as "non-deterministic" and therefore must be mocked in tests
type EntropySource interface {
	MakeUUID() string
	GetTime() time.Time
}

// RealEntropySource non-deterministic Entropy source that generates real payload
type RealEntropySource struct{}

func (_ *RealEntropySource) MakeUUID() string {
	return uuid.NewString()
}

func (_ *RealEntropySource) GetTime() time.Time {
	return time.Now()
}

type DeterministicGenerator struct {
	idx int
}

func (q *DeterministicGenerator) Poll() string {
	q.idx++
	return fmt.Sprintf("mock-%d", q.idx)
}

// StableEntropySource is an Entropy source that returns predefined mock values
type StableEntropySource struct {
	Generator DeterministicGenerator
	CurrTime  time.Time
}

func (e *StableEntropySource) MakeUUID() string {
	return e.Generator.Poll()
}

func (e *StableEntropySource) GetTime() time.Time {
	return e.CurrTime
}
