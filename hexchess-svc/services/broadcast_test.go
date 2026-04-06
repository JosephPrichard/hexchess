package svc

import (
	"context"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/util/logutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBroadcastGameMessage(t *testing.T) {
	t.Parallel()

	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	broadcasters := LocalBroadcasters{GamesCaster: MakeMultiCasterMap("testing-map", time.Hour*1)}
	<-broadcasters.ListenGameMessages(services.Redis)

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	wantMsgCount := 2

	subChan := make(chan []byte, wantMsgCount)
	broadcasters.GamesCaster.Subscribe("1", subChan)

	for _, input := range []struct {
		id  string
		msg string
	}{
		{id: "1", msg: "test1"},
		{id: "2", msg: "test3"},
		{id: "1", msg: "test2"},
	} {
		err := services.BroadcastGamesEvent(ctx, &pb.GameOutput{
			GameId: input.id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatMessage{Message: input.msg}},
		})
		require.NoError(t, err)
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
