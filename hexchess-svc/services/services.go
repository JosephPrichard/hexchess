package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/egress"
)

type Services struct {
	db.Postgres
	db.Redis
	egress.Aws
	egress.RemoteAPIs
	EntropySource
	LocalBroadcasters
}

func (svc *Services) Close() {
	if svc.Postgres != nil {
		svc.Postgres.Close()
	}
	svc.Redis.Close()
}
