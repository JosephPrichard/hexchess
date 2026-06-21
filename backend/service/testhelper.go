package svc

import (
	"hexchess-svc/cloud"
	"hexchess-svc/itest"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/pubsub"
)

type serviceMocks struct {
	Entropy     EntropyAPI
	Remote      cloud.RemoteAPIs
	S3Client    cloud.AWSClient
	Broadcaster *pubsub.Broadcaster
}

func setupServicesTest(t logutil.TestLogger, mocks *serviceMocks, flags ...itest.TestFlag) (*HexchessServices, itest.TestInfra) {
	if mocks == nil {
		mocks = &serviceMocks{}
	}

	infra := itest.SetupIntegrationTest(t, flags...)

	services := MakeHexchessServices(SetupService{
		DB:          infra.DB,
		Redis:       infra.Redis,
		Remote:      mocks.Remote,
		Entropy:     mocks.Entropy,
		Broadcaster: pubsub.MakeBroadcaster(infra.Redis),
		AWS:         infra.AWS,
	})

	return services, infra
}
