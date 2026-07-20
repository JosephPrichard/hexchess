package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/async"
)

type HexchessServices struct {
	database       db.Database[primarydb.Querier]
	querier        primarydb.Querier
	redis          db.Redis
	aws            cloud.AWSClient
	remote         cloud.RemoteAPIs
	broadcaster    *pubsub.Broadcaster
	redisPublisher producers.RedisPublisher
	entropy        EntropyAPI
	dispatcher     async.Dispatcher
}

type SetupService struct {
	PrimaryDB   db.Database[primarydb.Querier]
	Redis       db.Redis
	AWS         cloud.AWSClient
	Remote      cloud.RemoteAPIs
	Entropy     EntropyAPI
	Broadcaster *pubsub.Broadcaster
	Dispatcher  async.Dispatcher
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	var querier primarydb.Querier
	if setup.PrimaryDB != nil {
		querier = setup.PrimaryDB.Querier()
	}
	if setup.Entropy == nil {
		setup.Entropy = RealEntropySource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}
	return &HexchessServices{
		database:       setup.PrimaryDB,
		querier:        querier,
		redis:          setup.Redis,
		aws:            setup.AWS,
		remote:         setup.Remote,
		entropy:        setup.Entropy,
		redisPublisher: producers.NewPublisher(setup.Redis),
		broadcaster:    setup.Broadcaster,
		dispatcher:     setup.Dispatcher,
	}
}
