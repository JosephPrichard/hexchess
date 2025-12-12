package web

import "github.com/google/uuid"

type Generators interface {
	MakeID() string
}

type RandGenerator struct{}

func (_ *RandGenerator) MakeID() string {
	return uuid.NewString()
}

type mockGenerator struct {
	id string
}

func (m *mockGenerator) MakeID() string {
	return m.id
}
