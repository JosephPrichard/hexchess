package outbound

import (
	"github.com/google/uuid"
	"time"
)

type Generators interface {
	MakeID() string
	GetNow() time.Time
}

type RandGenerator struct{}

func (_ *RandGenerator) MakeID() string {
	return uuid.NewString()
}

func (_ *RandGenerator) GetNow() time.Time {
	return time.Now()
}

type MockGenerator struct {
	ID   string
	Time time.Time
}

func (g *MockGenerator) MakeID() string {
	return g.ID
}

func (g *MockGenerator) GetNow() time.Time {
	return g.Time
}
