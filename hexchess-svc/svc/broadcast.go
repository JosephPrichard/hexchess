package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"hexchess-svc/util"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func listenRedisChannels(rdb *redis.Client, chans []string, onMessage func(m *redis.Message)) chan struct{} {
	connCh := make(chan struct{})
	ctx := context.WithValue(context.Background(), util.Trace, fmt.Sprintf("redis-channel-listener-%v", chans))
	go func() {
		pubsub := rdb.Subscribe(ctx, chans...)
		defer pubsub.Close()

		slog.Info("starting channel subscriber", "channels", chans)

		ch := pubsub.Channel()
		connCh <- struct{}{}
		for msg := range ch {
			onMessage(msg)
		}
	}()
	return connCh
}

func ListenGameMessages(m *MultiCasterMap, rdb *db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubSub, []string{rdb.GamesChan}, func(v *redis.Message) {
		payload := []byte(v.Payload)
		var outputID pb.GameOutputID
		if err := proto.Unmarshal(payload, &outputID); err != nil {
			slog.Error("unmarshal game message", "err", err, "channel", v.Channel)
			return
		}
		slog.Info("received message on channel", "ID", outputID.GameId, "channel", v.Channel)
		go m.Broadcast(outputID.GameId, payload)
	})
}

func ListenUsersMessages(m *MultiCasterMap, rdb *db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubSub, []string{rdb.UsersChan}, func(v *redis.Message) {
		var userMsg pb.UserMsg
		if err := proto.Unmarshal([]byte(v.Payload), &userMsg); err != nil {
			slog.Error("unmarshal user message", "err", err, "channel", v.Channel)
			return
		}
		slog.Info("received message on channel", "userID", userMsg.UserId, "channel", v.Channel)

		buf, err := MarshalUserMsgJson(&userMsg)
		if err != nil {
			slog.Error("marshal user message", "err", err, "channel", v.Channel)
			return
		}
		m.Broadcast(userMsg.UserId, buf)
	})
}

func ListenUnicastEvents(m *UniCaster, rdb *db.Redis) chan struct{} {
	var eventMap = map[string]UcEventKind{
		rdb.ActiveCountChan: UcActiveEk,
		rdb.GamesCountChan:  UcGamesEk,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(rdb.PubSub, channels, func(v *redis.Message) {
		strData := v.Payload
		// unicast broadcasting is only being used to game counts (small data), so it is safe to log
		slog.Info("received event on channel", "event", strData, "channel", v.Channel)

		eKind, ok := eventMap[v.Channel]
		if !ok {
			slog.Error("received event on unmapped channel", "channel", v.Channel, "eventMap", eventMap)
			return
		}
		m.Broadcast(UcEvent{Kind: eKind, Data: strData})
	})
}

func BroadcastMessage(ctx context.Context, rdb *db.Redis, channel string, b []byte) error {
	res := rdb.PubSub.Publish(ctx, channel, b)
	if err := res.Err(); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel, "bytesCount", len(b))
	return nil
}

type CountEvent struct {
	ID    string `json:"id"`
	Count int64  `json:"count"`
}

func BroadcastCountEvent(ctx context.Context, rdb *db.Redis, channel string, count int64, id string) error {
	b, err := json.Marshal(CountEvent{ID: id, Count: count})
	if err != nil {
		return fmt.Errorf("marshal count event message: %w", err)
	}
	return BroadcastMessage(ctx, rdb, channel, b)
}

func BroadcastActiveCount(ctx context.Context, rdb *db.Redis, count int64, id string) error {
	return BroadcastCountEvent(ctx, rdb, rdb.ActiveCountChan, count, id)
}

func BroadcastGameCount(ctx context.Context, rdb *db.Redis, count int64, id string) error {
	return BroadcastCountEvent(ctx, rdb, rdb.GamesCountChan, count, id)
}

func BroadcastChallenge(ctx context.Context, rdb *db.Redis, id int64, c ChallengeEntity) error {
	um := SerializeChallengeMsg(id, c)
	b, err := proto.Marshal(um)
	if err != nil {
		return fmt.Errorf("marshal user challenge message: %w", err)
	}
	return BroadcastMessage(ctx, rdb, rdb.UsersChan, b)
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
		et := time.Since(br.GetLastAccess())
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
