package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/ext"
)

// Services is the information passed to any public service level API call (mocks, drivers, clients)
type Services struct {
	db.Postgres
	db.Redis
	ext.Aws
	ext.RemoteAPIs
	ext.EntropySource
	LocalBroadcasters
}

func (svc *Services) Close() {
	if svc.Postgres != nil {
		svc.Postgres.Close()
	}
	svc.Redis.Close()
}
