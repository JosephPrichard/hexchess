package pubsub

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"testing"
)

func TestBroadcastMessage(t *testing.T) {
	t.Parallel()

	redisAddr, _ := itest.SetupRedisTest(t.Context(), t)
	rdb := db.MakeRedis(
		db.RedisAddrs{
			GameStoreAddr: redisAddr,
			CacheAddr:     redisAddr,
			PubsubAddr:    redisAddr,
		},
		db.MakeTestRedisNames(),
	)
	defer rdb.Close()
	broadcaster := MakeBroadcaster(rdb)

	localBroadcasters := LocalBroadcasters{GamesCaster: MakeBroadcastActor("testing-multicaster")}
	<-localBroadcasters.ListenGameMessages(rdb)

	ctx := t.Context()

	wantMsgCount := 2

	subChan := make(chan []byte, wantMsgCount)
	localBroadcasters.GamesCaster.Subscribe("1", subChan)

	for _, input := range []struct {
		id  string
		msg string
	}{
		{id: "1", msg: "test1"},
		{id: "2", msg: "test3"},
		{id: "1", msg: "test2"},
	} {
		broadcaster.BroadcastGamesEvent(ctx, &pb.GameOutput{
			GameId: input.id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatMessage{Message: input.msg}},
		}, Sync())
	}

	var messages []string
	for range wantMsgCount {
		select {
		case <-ctx.Done():
			t.Errorf("test timed out: %v", ctx.Err())
		case v := <-subChan:
			messages = append(messages, mustUnmarshalChatMessage(t, v))
		}
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}

func mustUnmarshalChatMessage(t *testing.T, b []byte) string {
	t.Helper()

	var output pb.GameOutput
	require.NoError(t, proto.Unmarshal(b, &output))
	return output.GetChat().GetMessage()
}
