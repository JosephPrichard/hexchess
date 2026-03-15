package svc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/itest"
	"hexchess-svc/pb"
	"hexchess-svc/pkg/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiCaster(t *testing.T) {
	t.Parallel()

	// given
	m := MakeMultiCasterMap("testing-mc", time.Hour*1)

	wantSub1Msgs := []string{"test1", "test2", "test3"}
	wantSub2Msgs := []string{"test2", "test3"}
	wantSub3Msgs := []string{"test2"}
	wantSub4Msgs := []string{"test4"}

	sub1 := make(chan []byte, len(wantSub1Msgs))
	sub2 := make(chan []byte, len(wantSub2Msgs))
	sub3 := make(chan []byte, len(wantSub3Msgs))
	sub4 := make(chan []byte, len(wantSub4Msgs))

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)
	mChan3 := make(chan []string)
	mChan4 := make(chan []string)

	testSub := func(sub chan []byte, mChan chan []string) {
		var messages []string
		for msg := range sub {
			messages = append(messages, string(msg))
		}
		mChan <- messages
	}
	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)
	go testSub(sub3, mChan3)
	go testSub(sub4, mChan4)

	// when
	m.Subscribe("1", sub1)
	m.Subscribe("2", sub4)
	m.Broadcast("1", []byte("test1"))

	m.Subscribe("1", sub2)
	m.Subscribe("1", sub3)
	m.Broadcast("1", []byte("test2"))

	m.Unsubscribe("1", sub3)
	m.Broadcast("1", []byte("test3"))

	m.Broadcast("2", []byte("test4"))

	m.Unsubscribe("1", sub1)
	m.Unsubscribe("1", sub2)
	m.Unsubscribe("2", sub4)

	// then
	assert.Equal(t, wantSub1Msgs, <-mChan1)
	assert.Equal(t, wantSub2Msgs, <-mChan2)
	assert.Equal(t, wantSub3Msgs, <-mChan3)
	assert.Equal(t, wantSub4Msgs, <-mChan4)
}

func TestUnicaster(t *testing.T) {
	t.Parallel()

	// given
	m := MakeUniCaster("testing-uc")

	e1 := UcEvent{Kind: 0, Data: "test1"}
	e2 := UcEvent{Kind: 1, Data: "test2"}
	wantSub1Events := []UcEvent{e1, e2}
	wantSub2Events := []UcEvent{e2}

	sub1 := make(chan UcEvent, len(wantSub1Events))
	sub2 := make(chan UcEvent, len(wantSub2Events))

	mChan1 := make(chan []UcEvent)
	mChan2 := make(chan []UcEvent)

	testSub := func(sub chan UcEvent, mChan chan []UcEvent) {
		var messages []UcEvent
		for msg := range sub {
			messages = append(messages, msg)
		}
		mChan <- messages
	}
	go testSub(sub1, mChan1)
	go testSub(sub2, mChan2)

	// when
	m.Subscribe(sub1)
	m.Broadcast(e1)

	m.Subscribe(sub2)
	m.Broadcast(e2)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	// then
	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}

func TestBroadcastGameMessage(t *testing.T) {
	t.Parallel()

	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	lb := LocalBroadcasters{GamesCaster: MakeMultiCasterMap("testing-map", time.Hour*1)}
	<-lb.ListenGameMessages(services.Redis)

	ctx, cancel := context.WithTimeout(context.WithValue(t.Context(), logutil.Trace, t.Name()), 1*time.Second)
	defer cancel()

	wantMsgCount := 2

	// when
	subChan := make(chan []byte, wantMsgCount)
	lb.GamesCaster.Subscribe("1", subChan)

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
			var output pb.GameOutput
			require.NoError(t, proto.Unmarshal(v, &output))
			messages = append(messages, output.GetChat().GetMessage())
		}
	}
	assert.Equal(t, []string{"test1", "test2"}, messages)
}
