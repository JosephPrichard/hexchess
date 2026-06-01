package svc

import (
	"hexchess-svc/egress"
	"hexchess-svc/itest"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/pubsub"
)

type serviceMocks struct {
	Entropy     EntropyAPI
	Remote      egress.RemoteAPIs
	S3Client    egress.S3ClientAPI
	Broadcaster pubsub.BroadcasterAPI
}

func setupServicesTest(t logutil.TestLogger, mocks serviceMocks, flags ...itest.TestFlag) (*HexchessServices, itest.TestInfra) {
	infra := itest.SetupTestInfra(t, flags...)

	services := MakeHexchessServices(SetupService{
		DB:          infra.DB,
		Redis:       infra.Redis,
		AWS:         egress.AWS{S3Client: mocks.S3Client},
		Remote:      mocks.Remote,
		Entropy:     mocks.Entropy,
		Broadcaster: mocks.Broadcaster,
	})

	return services, infra
}
