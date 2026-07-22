package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
)

type HexchessServices struct {
	// postgres infra
	database      db.Database[primarydb.Querier]
	querier       primarydb.Querier
	riverProducer producers.RiverProducer
	// redis infra
	redis          db.Redis
	streamProducer producers.StreamProducer
	broadcaster    pubsub.Broadcaster
	// remote and local cloud sdks / clients
	aws  cloud.AWSClient
	sdks cloud.SDKs
	// non-deterministic behaviors for easy mocking
	entropy    entropy.Generator
	dispatcher async.Dispatcher
}

type SetupService struct {
	// postgres infra
	PrimaryDB   db.Database[primarydb.Querier]
	RiverClient producers.RiverClientAPI
	// redis infra
	Redis       db.Redis
	Broadcaster pubsub.Broadcaster
	// remote and local cloud sdks / clients
	AWS    cloud.AWSClient
	Remote cloud.SDKs
	// non-deterministic behaviors for easy mocking
	Entropy    entropy.Generator
	Dispatcher async.Dispatcher
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	var querier primarydb.Querier
	if setup.PrimaryDB != nil {
		querier = setup.PrimaryDB.Querier()
	}

	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	return &HexchessServices{
		database:      setup.PrimaryDB,
		querier:       querier,
		riverProducer: producers.NewRiverProducer(setup.RiverClient),

		redis:          setup.Redis,
		streamProducer: producers.NewPublisher(setup.Redis),
		broadcaster:    setup.Broadcaster,

		aws:  setup.AWS,
		sdks: setup.Remote,

		entropy:    setup.Entropy,
		dispatcher: setup.Dispatcher,
	}
}
