package web

import "github.com/google/uuid"

type Generators interface {
	MakeID() string
}

type UUIDGenerator struct{}

func (_ *UUIDGenerator) MakeID() string {
	return uuid.NewString()
}

type mockUUIDGenerator struct {
	id string
}

func (m *mockUUIDGenerator) MakeID() string {
	return m.id
}
