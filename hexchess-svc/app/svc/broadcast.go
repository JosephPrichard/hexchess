package svc

import (
	"context"
	"github.com/gomodule/redigo/redis"
	"github.com/puzpuzpuz/xsync/v4"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/pb"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const GamesChan = "games"

func BroadcastGameMessage(ctx context.Context, rdb *redis.Pool, message *pb.GameOutput) error {
	trace := ctx.Value(TraceKey)

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

var BrokerExpireTime = time.Hour

func StartListenGameMessages(rdb *redis.Pool) *xsync.Map[string, *MultiBroker] {
	brokers := xsync.NewMap[string, *MultiBroker]()

	go expireBrokers(brokers)
	go listenGameMessages(rdb, brokers)

	return brokers
}

func listenGameMessages(rdb *redis.Pool, m *xsync.Map[string, *MultiBroker]) {
	conn := rdb.Get()
	defer conn.Close()

	conn.Send("SUBSCRIBE", GamesChan)
	conn.Flush()

	slog.Info("starting channel broadcast subscriber", "channel", GamesChan)
	for {
		b, err := redis.Bytes(conn.Receive())
		if err != nil {
			slog.Error("failed to receive from games channel", "err", err)
			break
		}
		var goi pb.GameOutputID
		if err := proto.Unmarshal(b, &goi); err != nil {
			slog.Error("failed to unmarshal game message", "err", err)
			continue
		}

		br, ok := m.Load(goi.GameId)
		if ok && br != nil {
			br.Broadcast(b)
			slog.Info("broadcasted to broker", "GameID", goi.GameId)
		}
	}

	slog.Error("stopped broadcast channel subscriber", "channel", GamesChan)
}

func expireBrokers(brokers *xsync.Map[string, *MultiBroker]) {
	t := time.NewTicker(time.Minute * 1)
	for range t.C {
		brokers.Range(func(key string, br *MultiBroker) bool {
			if time.Now().Sub(br.GetLastAccess()) > BrokerExpireTime {
				brokers.Delete(key)
				slog.Info("expiring broker", "brokerID", key)
			}
			return true
		})
	}
}

func SubscribeBrokers(brokers *xsync.Map[string, *MultiBroker], brokerID string, sub Subscriber) {
	br, ok := brokers.Load(brokerID)
	if !ok {
		br = makeMultiBroker(brokerID)
		brokers.Store(brokerID, br)
	}
	br.Add(sub)
}

func UnsubscribeBrokers(m *xsync.Map[string, *MultiBroker], brokerID string, sub Subscriber) {
	if br, ok := m.Load(brokerID); ok {
		br.Remove(sub)
	}
}

type Subscriber = chan []byte

type MultiBroker struct {
	sync.RWMutex
	ID          string
	lastAccess  time.Time
	subscribers []Subscriber
}

func makeMultiBroker(brokerID string) *MultiBroker {
	return &MultiBroker{
		ID:         brokerID,
		lastAccess: time.Now(),
	}
}

func (br *MultiBroker) GetLastAccess() time.Time {
	br.RLock()
	defer br.RUnlock()

	return br.lastAccess
}

func (br *MultiBroker) Add(sub Subscriber) {
	br.Lock()
	defer br.Unlock()

	slog.Info("subscribing to broker", "brokerID", br.ID)
	br.lastAccess = time.Now()
	if !slices.Contains(br.subscribers, sub) {
		br.subscribers = append(br.subscribers, sub)
	}
}

func (br *MultiBroker) Remove(sub Subscriber) {
	br.Lock()
	defer br.Unlock()

	slog.Info("unsubscribing from broker", "brokerID", br.ID)
	br.lastAccess = time.Now()
	slices.DeleteFunc(br.subscribers, func(s Subscriber) bool { return s == sub })
	close(sub)
}

func (br *MultiBroker) Broadcast(msg []byte) {
	br.RLock()
	defer br.RUnlock()

	slog.Info("broadcasting to broker subscribers", "brokerID", br.ID)

	var dCount int
	for _, sub := range br.subscribers {
		select {
		case sub <- msg:
		default:
			dCount++
		}
	}
	if dCount > 0 {
		slog.Error("dropped send message to broker subscriber", "brokerID", br.ID, "dropCount", dCount)
	}
}

type SingleBroker struct {
	ID string
	xsync.Map[Subscriber, *atomic.Int64]
}

func (br *SingleBroker) Add(sub Subscriber) {
	slog.Info("subscribing to broker", "brokerID", br.ID, "sub", sub)
	t := &atomic.Int64{}
	t.Store(time.Now().UnixMilli())
	br.Store(sub, t)
}

func (br *SingleBroker) Remove(sub Subscriber) {
	slog.Info("unsubscribing from broker", "brokerID", br.ID, "sub", sub)
	br.Delete(sub)
	close(sub)
}

func (br *SingleBroker) Broadcast(msg []byte) {
	slog.Info("broadcasting to broker subscribers", "brokerID", br.ID)

	var dCount int

	br.Range(func(sub Subscriber, t *atomic.Int64) bool {
		lastAccess := time.UnixMilli(t.Load())
		if time.Now().Sub(lastAccess) > BrokerExpireTime {
			br.Remove(sub)
		} else {
			select {
			case sub <- msg:
			default:
				dCount++
			}
		}
		return true
	})
	if dCount > 0 {
		slog.Error("dropped send message to broker subscriber", "brokerID", br.ID, "dropCount", dCount)
	}
}
