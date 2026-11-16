package publishers

import (
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub/subscribers"
	"hexchess-svc/utils/goutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBroadcastMessage(t *testing.T) {
	ctx := t.Context()

	testInfra := itest.SetupIntegrationTest(t)
	rdb := testInfra.Redis

	defer rdb.Close()

	globalBroadcaster := NewBroadcaster(rdb, goutil.SyncDispatcher{})

	broadcasters := subscribers.NewLocalBroadcasters(subscribers.DefaultBrokerConfig)
	defer broadcasters.Shutdown()
	broadcasters.Listen(ctx, rdb)

	wantMsgCount := 2

	subscriber, unsubscribe := broadcasters.Games.Subscribe("1", wantMsgCount)
	defer unsubscribe()

	// note: there's a small race condition here where psc.Subscribe in ListenGameMessages may not observe broadcasts from "PUBLISH"
	// this is true even though psc.Subscribe is called before "PUBLISH" so the race condition is redis-side and unavoidable
	time.Sleep(1000 * time.Millisecond)

	for _, input := range []struct {
		id  string
		msg string
	}{
		{id: "1", msg: "test1"},
		{id: "2", msg: "test3"},
		{id: "1", msg: "test2"},
	} {
		globalBroadcaster.BroadcastGamesEvent(ctx, &pb.GameOutput{
			GameId: input.id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatMessage{Message: input.msg}},
		})
	}

	var messages []string
	for range wantMsgCount {
		messages = append(messages, mustUnmarshalChatMessage(t, <-subscriber.Mailbox()))
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}

func mustUnmarshalChatMessage(t *testing.T, b []byte) string {
	t.Helper()

	var output pb.GameOutput
	require.NoError(t, proto.Unmarshal(b, &output))
	return output.GetChat().GetMessage()
}
