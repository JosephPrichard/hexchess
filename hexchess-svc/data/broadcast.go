package data

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const GamesChan = "games"
const UsersChan = "users"
const GamesCountChan = "games_count"
const ActiveCountChan = "active_count"

func BroadcastMessage(ctx context.Context, rdb Rdb, channel string, b []byte) error {
	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, b); err != nil {
		slog.ErrorContext(ctx, "failed to publish message", "err", err)
		return err
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel, "bytesCount", len(b))
	return nil
}

func BroadcastActiveCount(ctx context.Context, rdb Rdb, count int64) error {
	return BroadcastMessage(ctx, rdb, rdb.ActiveCountChan, []byte(strconv.FormatInt(count, 10)))
}

func BroadcastGameCount(ctx context.Context, rdb Rdb, count int64) error {
	return BroadcastMessage(ctx, rdb, rdb.GamesCountChan, []byte(strconv.FormatInt(count, 10)))
}

func BroadcastChallenge(ctx context.Context, rdb Rdb, id int64, c ChallengeEntity) error {
	var pbUm pb.UserMessage
	mapPbChallengeMessage(&pbUm, id, c)

	b, err := proto.Marshal(&pbUm)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal user challenge message", "err", err)
		return err
	}
	return BroadcastMessage(ctx, rdb, rdb.UsersChan, b)
}

func makePubSub(conn redis.Conn, channel string) redis.PubSubConn {
	slog.Info("starting channel broadcast subscriber", "channel", channel)
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(channel)
	return psc
}

func ListenGameMessages(m *MultiCasterMap, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
		return
	}
	psc := makePubSub(conn, GamesChan)
	go listenGameMessages(psc, m)
}

func listenGameMessages(psc redis.PubSubConn, m *MultiCasterMap) {
	defer func() {
		psc.Close()
		slog.Error("stopped broadcast channel subscriber", "channel", GamesChan)
	}()
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			b := v.Data

			var goi pb.GameOutputID
			if err := proto.Unmarshal(b, &goi); err != nil {
				slog.Error("failed to unmarshal game message", "err", err, "channel", GamesChan)
				continue
			}
			slog.Info("received message on channel", "ID", goi.GameId, "channel", v.Channel)

			m.Broadcast(goi.GameId, b)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v, "channel", GamesChan)
		case error:
			slog.Error("failed to receive from games channel", "err", v, "channel", GamesChan)
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
	psc := makePubSub(conn, UsersChan)
	go listenUserMessages(psc, m)
}

func listenUserMessages(psc redis.PubSubConn, m *MultiCasterMap) {
	defer func() {
		psc.Close()
		slog.Error("stopped broadcast channel subscriber", "channel", UsersChan)
	}()
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			b := v.Data

			var um pb.UserMessage
			if err := proto.Unmarshal(b, &um); err != nil {
				slog.Error("failed to unmarshal user message", "err", err, "channel", UsersChan)
				continue
			}
			slog.Info("received message on channel", "userID", um.UserId, "channel", v.Channel)

			buf, err := MarshalUserMessage(&um)
			if err != nil {
				slog.Error("failed to marshal user message", "err", err, "channel", UsersChan)
				continue
			}
			m.Broadcast(um.UserId, buf)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v, "channel", UsersChan)
		case error:
			slog.Error("failed to receive from channel", "err", v, "channel", UsersChan)
			return
		}
	}
}

func ListenActiveCountsMessages(m *UniCaster, addr string) {
	ListenMessages(ActiveCountChan, m, addr)
}

func ListenGameCountsMessages(m *UniCaster, addr string) {
	ListenMessages(GamesCountChan, m, addr)
}

func ListenMessages(channel string, m *UniCaster, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
		return
	}
	psc := makePubSub(conn, channel)
	go listenMessages(channel, psc, m)
}

func listenMessages(channel string, psc redis.PubSubConn, m *UniCaster) {
	defer func() {
		psc.Close()
		slog.Error("stopped broadcast channel subscriber", "channel", channel)
	}()
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			slog.Info("received message on channel", "msg", string(v.Data), "channel", v.Channel)
			m.Broadcast(v.Data, BroadcasterExpireTime)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v, "channel", channel)
		case error:
			slog.Error("failed to receive from channel", "err", v, "channel", channel)
			return
		}
	}
}

type MultiCasterMap struct {
	ID string
	sync.RWMutex
	m map[string]*MultiCaster
}

func MakeMultiCasterMap(ID string) *MultiCasterMap {
	return &MultiCasterMap{
		ID: ID,
		m:  make(map[string]*MultiCaster),
	}
}

func (m *MultiCasterMap) Subscribe(brID string, sub subscriber) {
	slog.Info("subscribing to broker map", "mapID", m.ID, "brID", brID)

	m.Lock()
	defer m.Unlock()

	br, ok := m.m[brID]
	if !ok {
		br = MakeMultiCaster(brID)
		m.m[brID] = br
	}
	br.Subscribe(sub)
}

func (m *MultiCasterMap) Unsubscribe(brID string, sub subscriber) {
	didUnsub := false
	func() {
		m.RLock()
		defer m.RUnlock()

		if br, ok := m.m[brID]; ok {
			br.Unsubscribe(sub)
			didUnsub = true
		}
	}()
	if didUnsub {
		slog.Info("unsubscribed from broker map", "mapID", m.ID, "brID", brID)
	}
}

func (m *MultiCasterMap) Broadcast(brID string, msg []byte) {
	m.RLock()
	defer m.RUnlock()

	br, ok := m.m[brID]
	if ok && br != nil {
		br.Broadcast(msg)
	}
}

var BroadcasterExpireTime = time.Hour

func (m *MultiCasterMap) ExpirePeriodically(expireTime time.Duration) {
	for range time.NewTimer(time.Minute * 1).C {
		m.Expire(expireTime)
	}
}

func (m *MultiCasterMap) Expire(expireTime time.Duration) {
	type pair struct {
		key string
		br  *MultiCaster
	}
	var expiredBrs []pair // copy out so we can unsubscribe from channels outside the lock

	func() {
		m.Lock()
		defer m.Unlock()

		for key, br := range m.m {
			et := time.Now().Sub(br.GetLastAccess())
			if et > expireTime {
				expiredBrs = append(expiredBrs, pair{key, br})
				delete(m.m, key)
			}
		}
	}()

	slog.Info("expiring brokers", "expiredBrokers", expiredBrs)
	for _, p := range expiredBrs {
		p.br.UnsubscribeAll()
	}
}

type subscriber = chan []byte

type MultiCaster struct {
	sync.RWMutex
	ID          string
	lastAccess  atomic.Int64
	subscribers []subscriber
}

func MakeMultiCaster(brID string) *MultiCaster {
	br := &MultiCaster{ID: brID}
	br.lastAccess.Store(time.Now().UnixMilli())
	return br
}

func (br *MultiCaster) GetLastAccess() time.Time {
	return time.UnixMilli(br.lastAccess.Load())
}

func (br *MultiCaster) SetLastAccess() {
	br.lastAccess.Store(time.Now().UnixMilli())
}

func (br *MultiCaster) Subscribe(sub subscriber) {
	slog.Info("subscribing to broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	br.Lock()
	defer br.Unlock()

	br.SetLastAccess()
	if !slices.Contains(br.subscribers, sub) {
		br.subscribers = append(br.subscribers, sub)
	}
}

func (br *MultiCaster) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	br.SetLastAccess()

	br.Lock()
	defer br.Unlock()

	if slices.Contains(br.subscribers, sub) {
		close(sub)
		br.subscribers = slices.DeleteFunc(br.subscribers, func(s subscriber) bool { return s == sub })
	}
}

func (br *MultiCaster) UnsubscribeAll() {
	br.Lock()
	defer br.Unlock()

	for _, sub := range br.subscribers {
		close(sub)
	}
	br.subscribers = br.subscribers[:0]
}

func (br *MultiCaster) Broadcast(msg []byte) {
	var subscribers []subscriber // copy out so the sending doesn't keep the lock

	func() {
		br.RLock()
		defer br.RUnlock()
		for _, sub := range br.subscribers {
			subscribers = append(subscribers, sub)
		}
	}()

	slog.Info("broadcasting to broker subscribers", "brID", br.ID, "subscribers", subscribers)
	for _, sub := range subscribers {
		sub <- msg
	}
}

type UniCaster struct {
	ID string
	sync.RWMutex
	m map[subscriber]*atomic.Int64
}

func MakeUniCaster(brID string) *UniCaster {
	return &UniCaster{
		ID: brID,
		m:  make(map[subscriber]*atomic.Int64),
	}
}

func (br *UniCaster) Subscribe(sub subscriber) {
	slog.Info("subscribing to broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	br.Lock()
	defer br.Unlock()

	t := &atomic.Int64{}
	t.Store(time.Now().UnixMilli())
	br.m[sub] = t
}

func (br *UniCaster) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	br.Lock()
	defer br.Unlock()

	delete(br.m, sub)
	close(sub)
}

func (br *UniCaster) Broadcast(msg []byte, expireTime time.Duration) {
	var subscribers []subscriber // copy out so the sending doesn't keep the lock
	var expiredSubs []subscriber // copy this out so we don't log while lock is acquired

	func() {
		br.Lock()
		defer br.Unlock()

		for sub, t := range br.m {
			lastAccess := time.UnixMilli(t.Load())
			now := time.Now()
			if now.Sub(lastAccess) > expireTime {
				expiredSubs = append(expiredSubs, sub)
				delete(br.m, sub)
			} else {
				subscribers = append(subscribers, sub)
			}
			t.Store(now.UnixMilli())
		}
	}()

	slog.Info("expired subscribers", "brID", br.ID, "expiredSubs", expiredSubs)
	slog.Info("broadcasting to broker subscribers", "brID", br.ID, "count", len(subscribers))

	for _, sub := range subscribers {
		sub <- msg
	}
}
