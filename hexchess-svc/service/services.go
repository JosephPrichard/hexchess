package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/egress"
)

type HexchessServices struct {
	db      db.DB
	querier sqlc.Querier
	redis   db.Redis
	aws     egress.AWS
	remote  egress.RemoteAPIs
	entropy EntropySource
}

func (svc *HexchessServices) Close() {
	if svc.db != nil {
		svc.db.Close()
	}
	svc.redis.Close()
}

type Setup struct {
	DB      db.DB
	Querier sqlc.Querier
	Redis   db.Redis
	AWS     egress.AWS
	Remote  egress.RemoteAPIs
	Entropy EntropySource
}

func MakeHexchessServices(setup Setup) *HexchessServices {
	if setup.DB != nil {
		setup.Querier = setup.DB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = &RealEntropySource{}
	}
	return &HexchessServices{
		db:      setup.DB,
		querier: setup.Querier,
		redis:   setup.Redis,
		aws:     setup.AWS,
		remote:  setup.Remote,
		entropy: setup.Entropy,
	}
}
