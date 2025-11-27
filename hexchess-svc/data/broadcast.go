package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const GamesChan = "games"
const UsersChan = "users"
const GamesCountChan = "games_count"
const ActiveCountChan = "active_count"

func BroadcastMessage(ctx context.Context, rdb Redis, channel string, b []byte) error {
	conn := rdb.PubSub.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, b); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel, "bytesCount", len(b))
	return nil
}

type CountEvent struct {
	ID    string `json:"id"`
	Count int64  `json:"count"`
}

func BroadcastCountEvent(ctx context.Context, rdb Redis, channel string, count int64, id string) error {
	b, err := json.Marshal(CountEvent{ID: id, Count: count})
	if err != nil {
		return fmt.Errorf("failed to marshal count event message: %w", err)
	}
	return BroadcastMessage(ctx, rdb, channel, b)
}

func BroadcastActiveCount(ctx context.Context, rdb Redis, count int64, id string) error {
	return BroadcastCountEvent(ctx, rdb, rdb.ActiveCountChan, count, id)
}

func BroadcastGameCount(ctx context.Context, rdb Redis, count int64, id string) error {
	return BroadcastCountEvent(ctx, rdb, rdb.GamesCountChan, count, id)
}

func BroadcastChallenge(ctx context.Context, rdb Redis, id int64, c ChallengeEntity) error {
	um := SerializeChallengeMsg(id, c)
	b, err := proto.Marshal(&um)
	if err != nil {
		return fmt.Errorf("failed to marshal user challenge message: %w", err)
	}
	return BroadcastMessage(ctx, rdb, rdb.UsersChan, b)
}

func ListenGameMessages(m *MultiCasterMap, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
		return
	}
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(GamesChan)
	go listenGameMessages(psc, m)
}

func listenGameMessages(psc redis.PubSubConn, m *MultiCasterMap) {
	defer psc.Close()
	ch := GamesChan
	slog.Info("starting channel subscriber", "channel", ch)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			var goi pb.GameOutputID
			if err := proto.Unmarshal(v.Data, &goi); err != nil {
				slog.Error("failed to unmarshal game message", "err", err, "channel", ch)
				continue
			}
			slog.Info("received message on channel", "ID", goi.GameId, "channel", v.Channel)
			go m.Broadcast(goi.GameId, v.Data)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v, "channel", ch)
		case error:
			slog.Error("failed to receive from games channel", "err", v, "channel", ch)
			return
		}
	}
}

func ListenUsersMessages(m *MultiCasterMap, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
		return
	}
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(UsersChan)
	go listenUserMessages(psc, m)
}

func listenUserMessages(psc redis.PubSubConn, m *MultiCasterMap) {
	defer psc.Close()
	ch := UsersChan
	slog.Info("starting channel subscriber", "channel", ch)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			var um pb.UserMsg
			if err := proto.Unmarshal(v.Data, &um); err != nil {
				slog.Error("failed to unmarshal user message", "err", err, "channel", ch)
				continue
			}
			slog.Info("received message on channel", "userID", um.UserId, "channel", v.Channel)

			buf, err := MarshalUserMsgJson(&um)
			if err != nil {
				slog.Error("failed to marshal user message", "err", err, "channel", ch)
				continue
			}
			m.Broadcast(um.UserId, buf)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v, "channel", ch)
		case error:
			slog.Error("failed to receive from channel", "err", v, "channel", ch)
			return
		}
	}
}

var EventMap = map[string]UcEventKind{
	ActiveCountChan: UcActiveEk,
	GamesCountChan:  UcGamesEk,
}

func ListenUnicastEvents(m *UniCaster, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
		return
	}
	psc := redis.PubSubConn{Conn: conn}
	for ch := range EventMap {
		psc.Subscribe(ch)
	}
	go listenUnicastEvents(psc, m)
}

func listenUnicastEvents(psc redis.PubSubConn, m *UniCaster) {
	defer psc.Close()
	slog.Info("starting channel subscriber", "eventMap", EventMap)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			strData := string(v.Data)
			slog.Info("received event on channel", "event", strData, "channel", v.Channel) // unicast broadcasting is only being used to broadcastGameCounts counts (small data), so it is safe to log
			eKind, ok := EventMap[v.Channel]
			if !ok {
				slog.Error("received event on unmapped channel", "channel", v.Channel, "eventMap", EventMap)
				continue
			}
			m.Broadcast(UcEvent{Kind: eKind, Data: strData})
		case redis.Subscription:
			slog.Info("received subscription on channels", "value", v, "eventMap", EventMap)
		case error:
			slog.Error("failed to receive from channels", "err", v, "eventMap", EventMap)
			return
		}
	}
}

type MultiCasterMap struct {
	mu       sync.RWMutex
	ID       string
	m        map[string]*MultiCaster
	StopChan chan struct{}
}

func MakeMultiCasterMap(ID string, expireDuration time.Duration) *MultiCasterMap {
	stopChan := make(chan struct{})
	m := &MultiCasterMap{
		ID:       ID,
		m:        make(map[string]*MultiCaster),
		StopChan: stopChan,
	}
	if expireDuration > 0 {
		go m.ExpirePeriodically(expireDuration, stopChan)
	}
	return m
}

func (m *MultiCasterMap) Subscribe(brID string, sub chan []byte) {
	slog.Info("subscribing to multicaster map", "mapID", m.ID, "brID", brID)
	m.mu.Lock()
	br := m.m[brID]
	if br == nil {
		br = MakeMultiCaster(brID)
		m.m[brID] = br
	}
	m.mu.Unlock()
	br.Subscribe(sub)
}

func (m *MultiCasterMap) Unsubscribe(brID string, sub chan []byte) {
	m.mu.RLock()
	br := m.m[brID]
	m.mu.RUnlock()

	if br != nil {
		br.Unsubscribe(sub)
		slog.Info("unsubscribed from multicaster map", "mapID", m.ID, "brID", brID)
	}
}

func (m *MultiCasterMap) Broadcast(brID string, msg []byte) {
	m.mu.RLock()
	br, ok := m.m[brID]
	m.mu.RUnlock()

	if ok && br != nil {
		br.Broadcast(msg)
	}
}

var GameExpireDur = time.Hour

func (m *MultiCasterMap) ExpirePeriodically(expireDur time.Duration, stopChan chan struct{}) {
	ticker := time.NewTicker(time.Minute * 1)
	for {
		select {
		case <-ticker.C:
			m.Expire(expireDur)
		case <-stopChan:
			return
		}
	}
}

func (m *MultiCasterMap) Expire(expireDur time.Duration) {
	type pair struct {
		key string
		br  *MultiCaster
	}
	var expiredBrs []pair // copy out so we can remove channels outside the lock

	m.mu.Lock()
	for key, br := range m.m {
		et := time.Now().Sub(br.GetLastAccess())
		if et > expireDur {
			expiredBrs = append(expiredBrs, pair{key, br})
			delete(m.m, key)
		}
	}
	m.mu.Unlock()

	slog.Info("expiring multicasters from multicaster map", "mapID", m.ID, "expiredBrokers", expiredBrs)
	for _, p := range expiredBrs {
		p.br.UnsubscribeAll()
	}
}

type MultiCaster struct {
	mu          sync.RWMutex
	ID          string
	lastAccess  atomic.Int64
	subscribers []chan []byte
}

func MakeMultiCaster(brID string) *MultiCaster {
	br := &MultiCaster{ID: brID}
	br.lastAccess.Store(time.Now().UnixMilli())
	return br
}

func (mc *MultiCaster) GetLastAccess() time.Time {
	return time.UnixMilli(mc.lastAccess.Load())
}

func (mc *MultiCaster) SetLastAccess() {
	mc.lastAccess.Store(time.Now().UnixMilli())
}

func (mc *MultiCaster) Subscribe(sub chan []byte) {
	slog.Info("subscribing to multicaster", "brID", mc.ID, "sub", fmt.Sprintf("%v", sub))

	mc.SetLastAccess()

	mc.mu.Lock()
	if !slices.Contains(mc.subscribers, sub) {
		mc.subscribers = append(mc.subscribers, sub)
	}
	mc.mu.Unlock()
}

func (mc *MultiCaster) Unsubscribe(sub chan []byte) {
	slog.Info("unsubscribing from multicaster", "brID", mc.ID, "sub", fmt.Sprintf("%v", sub))

	mc.SetLastAccess()

	mc.mu.Lock()
	if slices.Contains(mc.subscribers, sub) {
		close(sub)
	}
	mc.subscribers = slices.DeleteFunc(mc.subscribers, func(s chan []byte) bool { return s == sub })
	mc.mu.Unlock()
}

func (mc *MultiCaster) UnsubscribeAll() {
	mc.mu.Lock()
	for _, sub := range mc.subscribers {
		close(sub)
	}
	mc.subscribers = mc.subscribers[:0]
	mc.mu.Unlock()
}

func (mc *MultiCaster) Broadcast(msg []byte) {
	var subStrs []string // copy out so logging doesn't keep the lock

	mc.mu.RLock()
	for _, sub := range mc.subscribers {
		sub <- msg
		subStrs = append(subStrs, fmt.Sprintf("%v", sub))
	}
	mc.mu.RUnlock()

	slog.Info("broadcasted to multicaster subscribers", "brID", mc.ID, "subscribers", subStrs)
}

type UcEventKind int

const (
	UcActiveEk UcEventKind = iota
	UcGamesEk
)

type UcEvent struct {
	Kind UcEventKind // event kind, an unicaster is used to broadcast all global event types in hexchess-svc
	Data string
}

type UniCaster struct {
	mu       sync.Mutex
	ID       string
	m        map[chan UcEvent]struct{}
	StopChan chan struct{}
}

func MakeUniCaster(brID string) *UniCaster {
	stopChan := make(chan struct{})
	br := &UniCaster{
		ID:       brID,
		m:        make(map[chan UcEvent]struct{}),
		StopChan: stopChan,
	}
	return br
}

func (uc *UniCaster) Subscribe(sub chan UcEvent) {
	slog.Info("subscribing to unicaster", "brID", uc.ID, "sub", fmt.Sprintf("%v", sub))

	uc.mu.Lock()
	uc.m[sub] = struct{}{}
	uc.mu.Unlock()
}

func (uc *UniCaster) Unsubscribe(sub chan UcEvent) {
	slog.Info("unsubscribing from unicaster", "brID", uc.ID, "sub", fmt.Sprintf("%v", sub))

	uc.mu.Lock()
	if _, ok := uc.m[sub]; ok {
		delete(uc.m, sub)
		close(sub)
	}
	uc.mu.Unlock()
}

func (uc *UniCaster) Broadcast(msg UcEvent) {
	count := 0

	uc.mu.Lock()
	for sub := range uc.m {
		count++
		sub <- msg
	}
	uc.mu.Unlock()

	slog.Info("broadcasted to unicaster subscribers", "brID", uc.ID, "count", count)
}
