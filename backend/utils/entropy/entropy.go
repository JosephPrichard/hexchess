package entropy

import (
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// Generator is a generator for generating things my program determines as "non-deterministic" and therefore must be mocked in tests
type Generator interface {
	NewUUID() uuid.UUID
	GetTime() time.Time
}

// RealSource non-deterministic Entropy source that generates real payload
type RealSource struct{}

func (_ RealSource) NewUUID() uuid.UUID {
	return uuid.New()
}

func (_ RealSource) GetTime() time.Time {
	return time.Now()
}

// StableSource is an Entropy source that returns predefined mock values
type StableSource struct {
	idx      uint64
	CurrTime time.Time
}

func (e *StableSource) NewUUID() uuid.UUID {
	var uid uuid.UUID
	binary.LittleEndian.PutUint64(uid[:], e.idx)
	e.idx++
	return uid
}

func (e *StableSource) GetTime() time.Time {
	return e.CurrTime
}
