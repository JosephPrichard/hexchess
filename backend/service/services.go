package svc

import (
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/egress"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
)

type HexchessServices struct {
	transactor     db.Transactor
	querier        sqlc.Querier
	redis          db.Redis
	aws            egress.AWSClient
	remote         egress.RemoteAPIs
	broadcaster    *pubsub.Broadcaster
	redisPublisher producers.RedisPublisher
	entropy        EntropyAPI
}

type SetupService struct {
	DB          db.DB
	Redis       db.Redis
	AWS         egress.AWSClient
	Remote      egress.RemoteAPIs
	Entropy     EntropyAPI
	Broadcaster *pubsub.Broadcaster
}

func MakeHexchessServices(setup SetupService) *HexchessServices {
	var querier sqlc.Querier
	if setup.DB != nil {
		querier = setup.DB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = &RealEntropySource{}
	}
	return &HexchessServices{
		transactor:     setup.DB,
		querier:        querier,
		redis:          setup.Redis,
		aws:            setup.AWS,
		remote:         setup.Remote,
		entropy:        setup.Entropy,
		redisPublisher: producers.MakePublisher(setup.Redis),
		broadcaster:    setup.Broadcaster,
	}
}
