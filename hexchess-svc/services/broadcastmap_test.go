package svc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMultiCasterMap(t *testing.T) {
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

	assert.Equal(t, wantSub1Msgs, <-mChan1)
	assert.Equal(t, wantSub2Msgs, <-mChan2)
	assert.Equal(t, wantSub3Msgs, <-mChan3)
	assert.Equal(t, wantSub4Msgs, <-mChan4)
}

func TestUnicasterMap(t *testing.T) {
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

	m.Subscribe(sub1)
	m.Broadcast(e1)

	m.Subscribe(sub2)
	m.Broadcast(e2)

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)

	assert.Equal(t, wantSub1Events, <-mChan1)
	assert.Equal(t, wantSub2Events, <-mChan2)
}
