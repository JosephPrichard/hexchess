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

func BroadcastMessage(ctx context.Context, rdb Redis, channel string, b []byte) error {
	conn := rdb.PubSub.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, b); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel, "bytesCount", len(b))
	return nil
}

func BroadcastActiveCount(ctx context.Context, rdb Redis, count int64) error {
	return BroadcastMessage(ctx, rdb, rdb.ActiveCountChan, []byte(strconv.FormatInt(count, 10)))
}

func BroadcastGameCount(ctx context.Context, rdb Redis, count int64) error {
	return BroadcastMessage(ctx, rdb, rdb.GamesCountChan, []byte(strconv.FormatInt(count, 10)))
}

func BroadcastChallenge(ctx context.Context, rdb Redis, id int64, c ChallengeEntity) error {
	pbUm := pb.UserMsg{
		UserId: strconv.Itoa(int(id)),
		Value: &pb.UserMsg_Challenge{Challenge: &pb.ChallengeMsg{
			ChallengerId:      c.ChallengerID,
			ChallengerName:    c.ChallengerName,
			ChallengerCountry: c.ChallengerCountry,
			ChallengerElo:     c.ChallengerElo,
			ChallengeeId:      c.ChallengeeID,
			ChallengeeName:    c.ChallengeeName,
			ChallengeeCountry: c.ChallengeeCountry,
			ChallengeeElo:     c.ChallengeeElo,
			TimeControl:       uint32(c.TimeControl),
			StartColor:        uint32(c.StartColor),
			MadeOn:            c.MadeOn.UnixMilli(),
		}},
	}
	b, err := proto.Marshal(&pbUm)
	if err != nil {
		return fmt.Errorf("failed to marshal user challenge message: %w", err)
	}
	return BroadcastMessage(ctx, rdb, rdb.UsersChan, b)
}

func makePubSub(conn redis.Conn, channel string) redis.PubSubConn {
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
		slog.Error("stopped channel subscriber", "channel", GamesChan)
	}()
	slog.Info("starting channel subscriber", "channel", GamesChan)
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
	slog.Info("starting channel subscriber", "channel", UsersChan)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			b := v.Data

			var um pb.UserMsg
			if err := proto.Unmarshal(b, &um); err != nil {
				slog.Error("failed to unmarshal user message", "err", err, "channel", UsersChan)
				continue
			}
			slog.Info("received message on channel", "userID", um.UserId, "channel", v.Channel)

			buf, err := MarshalUserMessageJson(&um)
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
	slog.Info("starting channel subscriber", "channel", channel)
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
	mu sync.RWMutex
	ID string
	m  map[string]*MultiCaster
}

func MakeMultiCasterMap(ID string) *MultiCasterMap {
	m := &MultiCasterMap{
		ID: ID,
		m:  make(map[string]*MultiCaster),
	}
	go m.ExpirePeriodically(BroadcasterExpireTime)
	return m
}

func (m *MultiCasterMap) Subscribe(brID string, sub subscriber) {
	slog.Info("subscribing to broker map", "mapID", m.ID, "brID", brID)

	m.mu.Lock()
	defer m.mu.Unlock()

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
		m.mu.RLock()
		defer m.mu.RUnlock()

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	br, ok := m.m[brID]
	if ok && br != nil {
		br.Broadcast(msg)
	}
}

var BroadcasterExpireTime = time.Hour

func (m *MultiCasterMap) ExpirePeriodically(expireTime time.Duration) {
	for range time.NewTicker(time.Minute * 1).C {
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
		m.mu.Lock()
		defer m.mu.Unlock()

		for key, br := range m.m {
			et := time.Now().Sub(br.GetLastAccess())
			if et > expireTime {
				expiredBrs = append(expiredBrs, pair{key, br})
				delete(m.m, key)
			}
		}
	}()

	slog.Info("expiring brokers from multicaster map", "mapID", m.ID, "expiredBrokers", expiredBrs)
	for _, p := range expiredBrs {
		p.br.UnsubscribeAll()
	}
}

type subscriber = chan []byte

type MultiCaster struct {
	mu          sync.RWMutex
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

	br.mu.Lock()
	defer br.mu.Unlock()

	br.SetLastAccess()
	if !slices.Contains(br.subscribers, sub) {
		br.subscribers = append(br.subscribers, sub)
	}
}

func (br *MultiCaster) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	br.SetLastAccess()

	br.mu.Lock()
	defer br.mu.Unlock()

	if slices.Contains(br.subscribers, sub) {
		close(sub)
		br.subscribers = slices.DeleteFunc(br.subscribers, func(s subscriber) bool { return s == sub })
	}
}

func (br *MultiCaster) UnsubscribeAll() {
	br.mu.Lock()
	defer br.mu.Unlock()

	for _, sub := range br.subscribers {
		close(sub)
	}
	br.subscribers = br.subscribers[:0]
}

func (br *MultiCaster) Broadcast(msg []byte) {
	var subStrs []string // copy out so logging doesn't keep the lock

	func() {
		br.mu.RLock()
		defer br.mu.RUnlock()
		for _, sub := range br.subscribers {
			sub <- msg
			subStrs = append(subStrs, fmt.Sprintf("%v", sub))
		}
	}()

	slog.Info("broadcasted to broker subscribers", "brID", br.ID, "subscribers", subStrs)
}

type UniCaster struct {
	mu sync.RWMutex
	ID string
	m  map[subscriber]*atomic.Int64
}

func MakeUniCaster(brID string) *UniCaster {
	br := &UniCaster{
		ID: brID,
		m:  make(map[subscriber]*atomic.Int64),
	}
	go br.ExpirePeriodically(BroadcasterExpireTime)
	return br
}

func (br *UniCaster) Subscribe(sub subscriber) {
	slog.Info("subscribing to broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))

	t := &atomic.Int64{}
	t.Store(time.Now().UnixMilli())

	br.mu.Lock()
	defer br.mu.Unlock()

	br.m[sub] = t
}

func (br *UniCaster) Unsubscribe(sub subscriber) {
	slog.Info("unsubscribing from broker", "brID", br.ID, "sub", fmt.Sprintf("%v", sub))
	
	br.mu.Lock()
	defer br.mu.Unlock()

	delete(br.m, sub)
	close(sub)
}

func (br *UniCaster) Broadcast(msg []byte, expireTime time.Duration) {
	subCount := 0
	var expiredSubs []subscriber // copy this out so we don't log while lock is acquired

	func() {
		br.mu.Lock()
		defer br.mu.Unlock()

		for sub, t := range br.m {
			lastAccess := time.UnixMilli(t.Load())
			now := time.Now()
			if now.Sub(lastAccess) > expireTime {
				expiredSubs = append(expiredSubs, sub)
				delete(br.m, sub)
			} else {
				sub <- msg
				subCount++
			}
			t.Store(now.UnixMilli())
		}
	}()

	slog.Info("expired subscribers", "brID", br.ID, "expiredSubs", expiredSubs)
	slog.Info("broadcasted to broker subscribers", "brID", br.ID, "count", subCount)
}

func (br *UniCaster) ExpirePeriodically(expireTime time.Duration) {
	for range time.NewTicker(time.Minute * 1).C {
		br.Expire(expireTime)
	}
}

func (br *UniCaster) Expire(expireTime time.Duration) {
	var expiredSubStrs []string // copy out so we can log outside the lock

	func() {
		br.mu.Lock()
		defer br.mu.Unlock()

		for sub, t := range br.m {
			et := time.Now().Sub(time.UnixMilli(t.Load()))
			if et > expireTime {
				delete(br.m, sub)
				close(sub)
				expiredSubStrs = append(expiredSubStrs, fmt.Sprintf("%v", sub))
			}
		}
	}()

	slog.Info("expired subscribers from unicaster", "brID", br.ID, "expiredSubs", expiredSubStrs)
}
