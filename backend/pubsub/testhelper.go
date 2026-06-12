package pubsub

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"hexchess-svc/db"
	"hexchess-svc/lib/testutil"
	"hexchess-svc/pb"
	"testing"
)

var cmpOptsGameOutputs cmp.Options = []cmp.Option{
	protocmp.Transform(),
	protocmp.IgnoreFields(&pb.Replay{}, "id", "played_on"),
}

func ExpectBroadcastGames(t *testing.T, rdb db.Redis, gameID string, wantOutputs []*pb.GameOutput) func() {
	if wantOutputs == nil {
		return func() {}
	}

	localBroadcasters := MakeLocalBroadcasters()
	localBroadcasters.Listen(rdb)

	subChan := make(chan []byte, len(wantOutputs))
	localBroadcasters.GamesCaster.Subscribe(gameID, subChan)

	return func() {
		for i := range wantOutputs {
			bytes := <-subChan

			var output pb.GameOutput
			require.NoError(t, proto.Unmarshal(bytes, &output))

			testutil.Equal(t, wantOutputs[i], &output, cmpOptsGameOutputs...)
		}
	}
}

func ExpectBroadcastActiveUsers(t *testing.T, rdb db.Redis, wantOutputs []int64) func() {
	if wantOutputs == nil {
		return func() {}
	}

	localBroadcasters := MakeLocalBroadcasters()
	localBroadcasters.Listen(rdb)

	subChan := make(chan GlobalCastEvent, len(wantOutputs))
	localBroadcasters.CountsCaster.Subscribe(subChan)

	return func() {
		for i := range wantOutputs {
			event := <-subChan

			var output CountEvent
			require.NoError(t, json.Unmarshal([]byte(event.Data), &output))

			assert.Equal(t, wantOutputs[i], output.Count)
		}
	}
}
