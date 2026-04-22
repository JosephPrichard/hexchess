package svc

import (
	"context"
	"hexchess-svc/egress"
	"testing"

	"hexchess-svc/itest"
	"hexchess-svc/util/logutil"
	"hexchess-svc/util/testutil"

	"github.com/google/go-cmp/cmp"
)

type Mocks struct {
	Entropy  EntropySource
	Remote   egress.RemoteAPIs
	S3Client egress.S3Client
}

func SetupServicesTest(t logutil.TestLogger, mocks Mocks, flags ...itest.TestFlag) (*HexchessServices, itest.TestInfra) {
	infra := itest.SetupTestInfra(t, flags...)

	aws := egress.AWS{S3Endpoint: "http://localhost:4566", S3Client: mocks.S3Client}

	services := MakeHexchessServices(Setup{
		DB:      infra.DB,
		Redis:   infra.Redis,
		AWS:     aws,
		Remote:  mocks.Remote,
		Entropy: mocks.Entropy,
	})

	return services, infra
}

func AssertRedisChessState(t *testing.T, services *HexchessServices, wantState *ChessState, options ...cmp.Option) {
	t.Helper()
	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	if wantState == nil {
		return
	}
	actualState, err := services.GetChessState(ctx, wantState.ID)
	if err != nil {
		t.Fatalf("failed to retrieve in redis chess state assert: %v", err)
	}
	testutil.Equal(t, wantState, actualState, options...)
}
