package pubsub

import (
	"hexchess-svc/cache"
	"hexchess-svc/itest"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"hexchess-svc/utils/config"
	"hexchess-svc/utils/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBroadcastMessage(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	redisAddr, _ := itest.SetupRedisTest(ctx, t)
	rdb := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   []string{redisAddr},
		PubsubAddr:    redisAddr,
		ActiveProfile: config.Local,

		Names: testutil.NewTestNames(cache.DefaultRedisNames),
	})
	defer rdb.Close()

	broadcaster := NewSyncBroadcaster(rdb)

	localBroadcasters := LocalBroadcasters{Games: NewBroadcastBroker[model.GameID]("testing-multicaster")}
	<-localBroadcasters.ListenGameMessages(rdb)

	wantMsgCount := 2

	subChan := make(chan []byte, wantMsgCount)
	localBroadcasters.Games.Subscribe("1", subChan)

	// note(Joseph): there's a small race condition here where psc.Subscribe in ListenGameMessages may not observe broadcasts from "PUBLISH"
	// this is true even though psc.Subscribe is called temporarily before "PUBLISH" so the race condition is redis-side and unavoidable
	time.Sleep(500 * time.Millisecond)

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
		})
	}

	var messages []string
	for range wantMsgCount {
		messages = append(messages, mustUnmarshalChatMessage(t, <-subChan))
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}

func mustUnmarshalChatMessage(t *testing.T, b []byte) string {
	t.Helper()

	var output pb.GameOutput
	require.NoError(t, proto.Unmarshal(b, &output))
	return output.GetChat().GetMessage()
}
