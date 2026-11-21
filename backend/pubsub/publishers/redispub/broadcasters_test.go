package redispub

import (
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/pubsub/subscribers/redisub"
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

	publisher := NewRedisPublisher(rdb.PubSubClient, goutil.SyncDispatcher{})

	broker := redisub.NewRedisBroadcastBroker(redisub.DefaultBrokerConfig, rdb.PubSubClient)
	defer broker.Close()
	broker.Start(ctx)

	wantMsgCount := 2

	subscriber, unsubscribe := broker.Games.Subscribe("1", wantMsgCount)
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
		o := &pb.GameOutput{
			GameId: input.id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatMessage{Message: input.msg}},
		}
		payload, err := o.MarshalVT()
		require.NoError(t, err)

		publisher.Publish(ctx, event.GamesChannel, o.GameId, payload)
	}

	var messages []string
	for range wantMsgCount {
		messages = append(messages, mustUnmarshalChatMessage(t, <-subscriber))
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}

func mustUnmarshalChatMessage(t *testing.T, b []byte) string {
	t.Helper()

	var output pb.GameOutput
	require.NoError(t, proto.Unmarshal(b, &output))
	return output.GetChat().GetMessage()
}
