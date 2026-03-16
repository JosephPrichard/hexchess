package svc

import (
	"context"
	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBroadcastGameMessage(t *testing.T) {
	// t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	broadcasters := LocalBroadcasters{GamesCaster: MakeMultiCasterMap("testing-map", time.Hour*1)}
	<-broadcasters.ListenGameMessages(services.Redis)

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())

	wantMsgCount := 2

	// when
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
		require.NoError(t, services.BroadcastGamesEvent(ctx, &pb.GameOutput{
			GameId: input.id,
			Value:  &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: input.msg}},
		}))
	}

	// then
	var messages []string
ReadMsgs:
	for range wantMsgCount {
		select {
		case <-ctx.Done():
			break ReadMsgs
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