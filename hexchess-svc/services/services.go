package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/egress"
)

type Services struct {
	DB            db.DB
	Queries       *sqlc.Queries
	Redis         db.Redis
    AWS           egress.AWS
	Remote        egress.RemoteAPIs
	EntropySource EntropySource
	Broadcasters  LocalBroadcasters
}

func (svc *Services) Close() {
	if svc.DB != nil {
		svc.DB.Close()
	}
	svc.Redis.Close()
}
