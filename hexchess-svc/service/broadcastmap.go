package svc

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

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
	var expiredBrs []pair // copy ext so we can remove channels outside the lock

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
	var subStrs []string // copy ext so logging doesn't keep the lock

	mc.mu.RLock()
	for _, sub := range mc.subscribers {
		select {
		case sub <- msg:
		default:
			// drop a message if the consumer is slow
		}
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
	Kind UcEventKind // event kind, an unicaster is used to broadcast all global event types in hexchess
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
		select {
		case sub <- msg:
		default:
			// drop a message if the consumer is slow
		}
	}
	uc.mu.Unlock()

	slog.Info("broadcasted to unicaster subscribers", "brID", uc.ID, "count", count)
}
