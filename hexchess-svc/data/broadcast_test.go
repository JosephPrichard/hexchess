package data

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/lib"
	"hexchess-svc/pb"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func testSub(t *testing.T, sub subscriber, mChan chan []string) {
	var messages []string
	for msg := range sub {
		messages = append(messages, string(msg))
	}
	t.Logf("completed testing subscriber: %v: %v", sub, messages)
	mChan <- messages
}

func TestMultiBroker(t *testing.T) {
	type msg = []byte

	m := MakeMultiCasterMap("testing-broker")

	sub1 := make(subscriber)
	sub2 := make(subscriber)
	sub3 := make(subscriber)
	sub4 := make(subscriber)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)
	mChan3 := make(chan []string)
	mChan4 := make(chan []string)

	go testSub(t, sub1, mChan1)
	go testSub(t, sub2, mChan2)
	go testSub(t, sub3, mChan3)
	go testSub(t, sub4, mChan4)

	m.Subscribe("1", sub1)
	m.Subscribe("2", sub4)
	m.Broadcast("1", msg("test1"))

	m.Subscribe("1", sub2)
	m.Subscribe("1", sub3)
	m.Broadcast("1", msg("test2"))

	m.Unsubscribe("1", sub3)
	m.Broadcast("1", msg("test3"))

	m.Broadcast("2", msg("test4"))

	m.Unsubscribe("1", sub1)
	m.Unsubscribe("1", sub2)
	m.Unsubscribe("2", sub4)

	assert.Equal(t, []string{"test1", "test2", "test3"}, <-mChan1)
	assert.Equal(t, []string{"test2", "test3"}, <-mChan2)
	assert.Equal(t, []string{"test2"}, <-mChan3)
	assert.Equal(t, []string{"test4"}, <-mChan4)
}

func TestSingleBroker(t *testing.T) {
	m := MakeUniCaster("testing-broker")

	sub1 := make(subscriber)
	sub2 := make(subscriber)

	mChan1 := make(chan []string)
	mChan2 := make(chan []string)

	go testSub(t, sub1, mChan1)
	go testSub(t, sub2, mChan2)

	m.Subscribe(sub1)
	m.Broadcast([]byte("test1"), BroadcasterExpireTime)

	m.Subscribe(sub2)
	m.Broadcast([]byte("test2"), BroadcasterExpireTime)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	assert.Equal(t, []string{"test1", "test2"}, <-mChan1)
	assert.Equal(t, []string{"test2"}, <-mChan2)
}

func getChatMessages(t *testing.T, bytes [][]byte) []string {
	var msgs []string
	for _, b := range bytes {
		var output pb.GameOutput
		assert.NoError(t, proto.Unmarshal(b, &output))
		msgs = append(msgs, output.GetChat().GetMessage())
	}
	return msgs
}

func testCountSub(t *testing.T, sub subscriber, mChan chan [][]byte, count int) {
	var messages [][]byte
	for range count {
		msg := <-sub
		messages = append(messages, msg)
	}
	t.Logf("completed testing subscriber: %v: %v", sub, messages)
	mChan <- messages
}

func makeTestChatOutput(t *testing.T, id string, msg string) []byte {
	b, err := proto.Marshal(&pb.GameOutput{
		GameId: id,
		Value:  &pb.GameOutput_Chat{Chat: &pb.ChatOutput{Message: msg}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBroadcastGameMessage(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close() // this will also stop the goroutine listening to the pubsub channel

	m := MakeMultiCasterMap("testing-broker-map")
	ListenGameMessages(m, rdb.Addr)

	ctx := context.WithValue(context.Background(), lib.TK, "testing-broadcast-game-message")

	sub := make(subscriber)
	mChan := make(chan [][]byte)
	go testCountSub(t, sub, mChan, 2)
	m.Subscribe("1", sub)

	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "1", "test1")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "1", "test2")))
	assert.NoError(t, BroadcastMessage(ctx, rdb, GamesChan, makeTestChatOutput(t, "2", "test3")))

	assert.Equal(t, []string{"test1", "test2"}, getChatMessages(t, <-mChan))
}

func makeTestMoveOutput(b *testing.B, gID string, mID string) []byte {
	pbGame, err := MapPbGame(chess.MakeStartGame())
	if err != nil {
		b.Fatal(err)
	}
	bytes, err := proto.Marshal(&pb.GameOutput{
		GameId:    gID,
		MessageId: mID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				PieceMove: &pb.PieceMove{Piece: int32(chess.WhitePawn), ToFile: int32(1), ToRank: int32(1), FromFile: int32(2), FromRank: int32(1)},
				Game:      pbGame,
			},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	return bytes
}

func BenchmarkBroadcastGameMessage(b *testing.B) {
	b.N = 100

	const benchBrCount = 5
	const benchSubCount = 3 // subs per broker

	rdb := BeforeRedisTests(b)
	rdb.Close() // this will also stop the goroutine listening to the pubsub channel

	m := MakeMultiCasterMap("testing-broker-map")
	ListenGameMessages(m, rdb.Addr)

	ctx := context.WithValue(context.Background(), lib.TK, "testing-broadcast-game-message")

	var mapMu sync.Mutex
	sends := make(map[string]time.Time)
	recvs := make(map[string]time.Time)

	consistentSub := func(sub subscriber, brID string) {
		sID := uuid.NewString()
		for msg := range sub {
			var goi pb.GameOutputID
			if err := proto.Unmarshal(msg, &goi); err != nil {
				b.Fatal(err)
			}
			t := time.Now()
			func() {
				mapMu.Lock()
				defer mapMu.Unlock()
				recvs[goi.MessageId] = t
			}()
			b.Logf("receiving mID %s, brID %s, sID: %s, time: %d", goi.MessageId, brID, sID, t.UnixMicro())
		}
	}

	//intermittentSub := func(brID string) {
	//	// simulates a subscriber that is intermittently subscribing and unsubscribing
	//	var sub subscriber
	//	for range time.NewTimer(time.Second).C {
	//		if sub != nil {
	//			m.Unsubscribe(brID, sub)
	//			sub = nil
	//		} else {
	//			sub = make(subscriber)
	//			m.Subscribe(brID, sub)
	//		}
	//	}
	//}

	var testBrokers []string
	for range benchBrCount {
		brID := uuid.NewString()
		testBrokers = append(testBrokers, brID)
		for range benchSubCount {
			sub := make(subscriber)
			m.Subscribe(brID, sub)
			go consistentSub(sub, brID)
		}
		//for range benchSubCount {
		//	go intermittentSub(brID)
		//}
	}

	for range b.N {
		mID := uuid.NewString()
		brID := testBrokers[rand.Intn(len(testBrokers))]
		output := makeTestMoveOutput(b, brID, mID)

		t := time.Now()
		sends[mID] = t
		b.Logf("sending mID: %s, brID: %s, time: %d", mID, brID, t.UnixMicro())

		b.StartTimer()

		assert.NoError(b, BroadcastMessage(ctx, rdb, GamesChan, output))

		b.StopTimer()
	}

	var totalTime int64
	var count int64

	for k, v := range sends {
		v1, ok := recvs[k]
		if !ok {
			b.Fatalf("missing matching recv for mID %s", k)
		}
		diff := v1.Sub(v)
		totalTime += diff.Nanoseconds()
		count++
		b.Logf("mID: %s, time: %v", k, diff)
	}

	b.Logf("total time: %v", time.Duration(totalTime)*time.Nanosecond)
	b.Logf("avg time: %v", time.Duration(int(totalTime/count))*time.Nanosecond)
}
