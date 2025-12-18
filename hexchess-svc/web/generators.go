package web

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

type stableGenerator struct {
	id   string
	time time.Time
}

func (g *stableGenerator) MakeID() string {
	return g.id
}

func (g *stableGenerator) GetNow() time.Time {
	return g.time
}
