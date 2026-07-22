package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/pubsub"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/logutil"
)

type serviceMocks struct {
	Entropy     entropy.Generator
	Remote      cloud.SDKs
	S3Client    cloud.AWSClient
	Broadcaster *pubsub.Broadcaster
}

func setupServicesTest(t logutil.TestLogger, mocks *serviceMocks, flags ...itest.TestFlag) (*HexchessServices, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}

	infra := itest.SetupIntegrationTest(t, flags...)

	services := NewHexchessServices(SetupService{
		PrimaryDB:   infra.PrimaryDB,
		Redis:       infra.Redis,
		Remote:      mocks.Remote,
		Entropy:     mocks.Entropy,
		Broadcaster: pubsub.NewSyncBroadcaster(infra.Redis),
		AWS:         infra.AWS,
	})

	return services, infra
}
