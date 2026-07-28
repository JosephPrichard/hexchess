package svc

import (
	"hexchess-svc/cache"
	"hexchess-svc/cloud"
	"hexchess-svc/db"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/async"
	"hexchess-svc/utils/entropy"
)

type HexchessServices struct {
	// postgres infra
	database      db.Database
	querier       db.ReadWriteQuerier
	readQuerier   db.ReadQuerier
	riverProducer producers.RiverProducer
	// redis infra
	redis          cache.Redis
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
	Database    db.Database
	RiverClient producers.RiverClientAPI
	// redis infra
	Redis       cache.Redis
	Broadcaster pubsub.Broadcaster
	// remote and local cloud sdks / clients
	AWS    cloud.AWSClient
	Remote cloud.SDKs
	// non-deterministic behaviors for easy mocking
	Entropy    entropy.Generator
	Dispatcher async.Dispatcher
}

func NewHexchessServices(setup SetupService) *HexchessServices {
	var rwQuerier db.ReadWriteQuerier
	var roQuerier db.ReadQuerier

	if setup.Database != nil {
		rwQuerier = setup.Database.Querier()
		roQuerier = setup.Database.ReadQuerier()
	}

	if setup.Entropy == nil {
		setup.Entropy = entropy.RealSource{}
	}
	if setup.Dispatcher == nil {
		setup.Dispatcher = async.AsyncDispatcher{}
	}

	return &HexchessServices{
		database:      setup.Database,
		querier:       rwQuerier,
		readQuerier:   roQuerier,
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
