package pubsub

import (
	"encoding/json"
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

var cmpOptsGameOutputs cmp.Options = []cmp.Option{
	protocmp.Transform(),
	protocmp.IgnoreFields(&pb.ReplayOutput{}, "id", "played_on"),
}

func ExpectBroadcastGames(t *testing.T, rdb cache.Redis, gameID model.GameID, wantOutputs []*pb.GameOutput) func() {
	if wantOutputs == nil {
		return func() {}
	}

	localBroadcasters := NewLocalBroadcasters()
	localBroadcasters.Listen(rdb)

	subChan := make(chan []byte, len(wantOutputs))
	localBroadcasters.Games.Subscribe(gameID, subChan)

	return func() {
		for i := range wantOutputs {
			var bytes []byte
			select {
			case bytes = <-subChan:
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for broadcaster channel")
			}

			var output pb.GameOutput
			require.NoError(t, proto.Unmarshal(bytes, &output))

			testutil.Equal(t, wantOutputs[i], &output, cmpOptsGameOutputs...)
		}
	}
}

func ExpectBroadcastActiveUsers(t *testing.T, rdb cache.Redis, wantOutputs []int64) func() {
	if wantOutputs == nil {
		return func() {}
	}

	localBroadcasters := NewLocalBroadcasters()
	localBroadcasters.Listen(rdb)

	subChan := make(chan GlobalCastEvent, len(wantOutputs))
	localBroadcasters.Counts.Subscribe(subChan)

	return func() {
		for i := range wantOutputs {
			event := <-subChan

			var output CountEvent
			require.NoError(t, json.Unmarshal([]byte(event.Data), &output))

			assert.Equal(t, wantOutputs[i], output.Count)
		}
	}
}
