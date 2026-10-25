package pubsub

import (
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func expectBroadcast[Message any](
	t *testing.T,
	rdb cache.Redis,
	want []Message,
	subscribe func(lb *LocalBroadcasters, ch chan []byte),
	unmarshal func(bytes []byte) Message,
	cmpOpts ...cmp.Option,
) func() {
	t.Helper()

	if want == nil {
		return func() {}
	}

	lb := NewLocalBroadcasters()
	lb.Listen(rdb)

	subChan := make(chan []byte, len(want))
	subscribe(lb, subChan)

	return func() {
		t.Helper()

		for i := range want {
			var raw []byte
			select {
			case raw = <-subChan:
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for broadcaster message %d/%d", i+1, len(want))
			}

			got := unmarshal(raw)

			testutil.Equal(t, want[i], got, cmpOpts...)
		}
	}
}

func ExpectBroadcastGames(t *testing.T, rdb cache.Redis, gameID model.GameID, wantOutputs []*pb.GameOutput) func() {
	cmpOptsGameOutputs := []cmp.Option{
		protocmp.Transform(),
		protocmp.IgnoreFields(&pb.ReplayOutput{}, "id", "played_on"),
	}

	return expectBroadcast(
		t, rdb, wantOutputs,
		func(lb *LocalBroadcasters, ch chan []byte) { lb.Games.Subscribe(gameID, ch) },
		func(bytes []byte) *pb.GameOutput {
			o := &pb.GameOutput{}
			require.NoError(t, proto.Unmarshal(bytes, o))
			return o
		},
		cmpOptsGameOutputs...,
	)
}

func ExpectBroadcastTournamentOutput(t *testing.T, rdb cache.Redis, tournamentKey string, wantOutputs []model.TournamentOutput) func() {
	cmpOptsFullMatch := cmpopts.IgnoreFields(model.FullMatch{}, "GameID", "CreatedOn")

	return expectBroadcast(
		t, rdb, wantOutputs,
		func(lb *LocalBroadcasters, ch chan []byte) { lb.Tournament.Subscribe(tournamentKey, ch) },
		func(bytes []byte) model.TournamentOutput {
			var o model.TournamentOutput
			require.NoError(t, sonic.Unmarshal(bytes, &o))
			return o
		},
		cmpOptsFullMatch,
	)
}
