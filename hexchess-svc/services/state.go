package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/out"
)

// State is the information passed to any public service level API call (mocks, drivers, clients)
type State struct {
	*db.Postgres
	*db.Redis
	out.EntropySource
}

func (s State) Close() {
	if s.Postgres != nil {
		s.Postgres.Close()
	}
	if s.Redis != nil {
		s.Redis.Close()
	}
}
