package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/logutil"
)

type serviceMocks struct {
	RiverClient producers.RiverClientAPI
	Broadcaster *pubsub.Broadcaster
	S3Client    cloud.AWSClient
	Entropy     entropy.Generator
	Remote      cloud.SDKs
}

func setupServicesTest(t logutil.TestLogger, mocks *serviceMocks, flags ...itest.TestFlag) (*HexchessServices, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}
	if mocks.RiverClient == nil {
		mocks.RiverClient = producers.NoopRiverClient{}
	}

	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewHexchessServices(SetupService{
		Database:    infra.Database,
		RiverClient: mocks.RiverClient,
		Redis:       infra.Redis,
		Broadcaster: pubsub.NewSyncBroadcaster(infra.Redis),
		AWS:         infra.AWS,
		SDKs:        mocks.Remote,
		Entropy:     mocks.Entropy,
	})

	return services, infra
}
