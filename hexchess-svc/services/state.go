package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/ext"
)

// State is the information passed to any public service level API call (mocks, drivers, clients)
type State struct {
	db.Postgres
	db.Redis
	ext.Aws
	ext.RemoteAPIs
	ext.EntropySource
	LocalBroadcasters
}

func (s *State) Close() {
	if s.Postgres != nil {
		s.Postgres.Close()
	}
	s.Redis.Close()
}
