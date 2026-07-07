package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
)

type HexchessServices struct {
	database       db.Database
	querier        sqlc.Querier
	redis          db.Redis
	aws            cloud.AWSClient
	remote         cloud.RemoteAPIs
	broadcaster    *pubsub.Broadcaster
	redisPublisher producers.RedisPublisher
	entropy        EntropyAPI
}

type SetupService struct {
	DB          db.Database
	Redis       db.Redis
	AWS         cloud.AWSClient
	Remote      cloud.RemoteAPIs
	Entropy     EntropyAPI
	Broadcaster *pubsub.Broadcaster
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	var querier sqlc.Querier
	if setup.DB != nil {
		querier = setup.DB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = RealEntropySource{}
	}
	return &HexchessServices{
		database:       setup.DB,
		querier:        querier,
		redis:          setup.Redis,
		aws:            setup.AWS,
		remote:         setup.Remote,
		entropy:        setup.Entropy,
		redisPublisher: producers.NewPublisher(setup.Redis),
		broadcaster:    setup.Broadcaster,
	}
}
