package svc

import (
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// EntropyAPI is a generator for generating things my program determines as "non-deterministic" and therefore must be mocked in tests
type EntropyAPI interface {
	MakeUUID() uuid.UUID
	GetTime() time.Time
}

// RealEntropySource non-deterministic Entropy source that generates real payload
type RealEntropySource struct{}

func (_ *RealEntropySource) MakeUUID() uuid.UUID {
	return uuid.New()
}

func (_ *RealEntropySource) GetTime() time.Time {
	return time.Now()
}

// StableEntropySource is an Entropy source that returns predefined mock values
type StableEntropySource struct {
	idx      uint64
	CurrTime time.Time
}

func (e *StableEntropySource) MakeUUID() uuid.UUID {
	var uid uuid.UUID
	binary.LittleEndian.PutUint64(uid[:], e.idx)
	e.idx++
	return uid
}

func (e *StableEntropySource) GetTime() time.Time {
	return e.CurrTime
}
