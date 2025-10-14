package data

import (
	"context"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/app/util"
	"hexchess-svc/pb"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const GamesChan = "games"

func BroadcastGameMessage(ctx context.Context, rdb *redis.Pool, message *pb.GameOutput) error {
	trace := ctx.Value(util.TraceKey)
	b, err := proto.Marshal(message)
	if err != nil {
		slog.Error("failed to marshal game message", "msg", message, "err", err, "trace", trace)
		return err
	}

	conn := rdb.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", GamesChan, b); err != nil {
		slog.Error("failed to publish message", "err", err, "trace", trace)
		return err
	}
	slog.Info("broadcasted message to channel", "msg", message, "trace", trace)
	return nil
}

func DialAndListenGameMessages(m *MultiBrokerMap, addr string) {
	conn, err := redis.Dial("tcp", addr)
	if err != nil {
		slog.Error("failed to get conn for pubsub", "err", err)
	}

	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(GamesChan)

	go listenGameMessages(psc, m)
}

func listenGameMessages(psc redis.PubSubConn, m *MultiBrokerMap) {
	slog.Info("starting channel broadcast subscriber", "channel", GamesChan)
listenLoop:
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			b := v.Data

			var goi pb.GameOutputID
			if err := proto.Unmarshal(b, &goi); err != nil {
				slog.Error("failed to unmarshal game message", "err", err)
				continue
			}
			slog.Info("received message on channel", "ID", goi.GameId, "channel", GamesChan)

			m.Broadcast(goi.GameId, b)
		case redis.Subscription:
			slog.Info("received subscription on channel", "value", v)
		case error:
			slog.Error("failed to receive from games channel", "err", v)
			break listenLoop
		}
	}
	psc.Close()
	slog.Error("stopped broadcast channel subscriber", "channel", GamesChan)
}

type MultiBrokerMap struct {
	ID string
	sync.RWMutex
	m map[string]*MultiBroker
}

func MakeMultiBrokerMap(ID string) *MultiBrokerMap {
	return &MultiBrokerMap{
		ID: ID,
		m:  make(map[string]*MultiBroker),
	}
}

func (m *MultiBrokerMap) Subscribe(brokerID string, sub subscriber) {
	br, ok := m.m[brokerID]
	if !ok {
		br = MakeMultiBroker(brokerID)
		m.m[brokerID] = br
	}
	br.Subscribe(sub)
	slog.Info("subscribed to broker map", "mapID", m.ID, "brokerID", brokerID)
}

func (m *MultiBrokerMap) Unsubscribe(brokerID string, sub subscriber) {
	if br, ok := m.m[brokerID]; ok {
		br.Unsubscribe(sub)
		slog.Info("unsubscribed from broker map", "mapID", m.ID, "brokerID", brokerID)
	}
}

func (m *MultiBrokerMap) Broadcast(brokerID string, msg []byte) {
	br, ok := m.m[brokerID]
	if ok && br != nil {
		br.Broadcast(msg)
	}
}

var BrokerExpireTime = time.Hour

func (m *MultiBrokerMap) ExpirePeriodically(expireTime time.Duration) {
	for range time.NewTimer(time.Minute * 1).C {
		m.Expire(expireTime)
	}
}

func (m *MultiBrokerMap) Expire(expireTime time.Duration) {
	type pair struct {
		key string
		br  *MultiBroker
	}
	var expiredBrokers []pair // copy out so we can unsubscribe from channels outside the lock

	func() {
		m.RLock()
		defer m.RUnlock()

		for key, br := range m.m {
			et := time.Now().Sub(br.GetLastAccess())
			if et > expireTime {
				expiredBrokers = append(expiredBrokers, pair{key, br})
				delete(m.m, key)
			}
		}
	}()

	slog.Info("expiring brokers", "expiredBrokers", expiredBrokers)
	for _, p := range expiredBrokers {
		p.br.UnsubscribeAll()
	}
}

type subscriber = chan []byte

type MultiBroker struct {
	sync.RWMutex
	ID          string
	lastAccess  atomic.Int64
	subscribers []subscriber
}

func MakeMultiBroker(brokerID string) *MultiBroker {
	br := &MultiBroker{ID: brokerID}
	br.lastAccess.Store(time.Now().UnixMilli())
	return br
}

func (br *MultiBroker) GetLastAccess() time.Time {
	return time.UnixMilli(br.lastAccess.Load())
}

func (br *MultiBroker) SetLastAccess() {
	br.lastAccess.Store(time.Now().UnixMilli())
}

func (br *MultiBroker) Subscribe(sub subscriber) {
	slog.Info("subscribing to broker", "brokerID", br.ID, "sub", sub)

	br.Lock()
	defer br.Unlock()

	br.SetLastAccess()
	if !slices.Contains(br.subscribers, sub) {
		br.subscribers = append(br.subscribers, sub)
	}
}

func (br *MultiBroker) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brokerID", br.ID, "sub", sub)

	br.Lock()
	defer br.Unlock()

	br.SetLastAccess()
	if slices.Contains(br.subscribers, sub) {
		close(sub)
	}
	br.subscribers = slices.DeleteFunc(br.subscribers, func(s subscriber) bool { return s == sub })
}

func (br *MultiBroker) UnsubscribeAll() {
	br.Lock()
	defer br.Unlock()

	for _, sub := range br.subscribers {
		close(sub)
	}
	br.subscribers = br.subscribers[:0]
}

func (br *MultiBroker) Broadcast(msg []byte) {
	var subscribers []subscriber // copy out so the sending doesn't keep the lock

	func() {
		br.RLock()
		defer br.RUnlock()
		for _, sub := range br.subscribers {
			subscribers = append(subscribers, sub)
		}
	}()

	slog.Info("broadcasting to broker subscribers", "brokerID", br.ID, "subscribers", subscribers)
	for _, sub := range br.subscribers {
		sub <- msg
	}
}

type SingleBroker struct {
	ID string
	sync.RWMutex
	m map[subscriber]*atomic.Int64
}

func MakeSingleBroker(brokerID string) *SingleBroker {
	return &SingleBroker{
		ID: brokerID,
		m:  make(map[subscriber]*atomic.Int64),
	}
}

func (br *SingleBroker) Subscribe(sub subscriber) {
	slog.Info("subscribing to broker", "brokerID", br.ID, "sub", sub)

	br.Lock()
	defer br.Unlock()

	t := &atomic.Int64{}
	t.Store(time.Now().UnixMilli())
	br.m[sub] = t
}

func (br *SingleBroker) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brokerID", br.ID, "sub", sub)

	br.Lock()
	defer br.Unlock()

	delete(br.m, sub)
	close(sub)
}

func (br *SingleBroker) Broadcast(msg []byte, expireTime time.Duration) {
	var subscribers []subscriber // copy out so the sending doesn't keep the lock
	var expiredSubs []subscriber // copy this out so we don't log while lock is acquired

	func() {
		br.RLock()
		defer br.RUnlock()

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

	slog.Info("expired subscribers", "brokerID", br.ID, "expiredSubs", expiredSubs)
	slog.Info("broadcasting to broker subscribers", "brokerID", br.ID, "count", len(subscribers))

	for _, sub := range subscribers {
		sub <- msg
	}
}
