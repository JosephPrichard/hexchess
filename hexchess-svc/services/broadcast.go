package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/pb"
	"log/slog"
	"strconv"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
)

func listenRedisChannels(addr string, chans []string, onMessage func(m redigo.Message)) chan struct{} {
	connCh := make(chan struct{}, 1) // send a signal whenever the connection is complete
	recvLoop := func(conn redigo.Conn) {
		psc := redigo.PubSubConn{Conn: conn}
		defer psc.Close()
		for _, ch := range chans {
			psc.Subscribe(ch)
		}
		slog.Info("starting channel subscriber", "channels", chans)

		// signals to the caller whenever the background routine is *actually* listening on the channels
		connCh <- struct{}{}
		for {
			switch v := psc.Receive().(type) {
			case redigo.Message:
				onMessage(v)
			case redigo.Subscription:
				slog.Info("received subscription on channel", "value", v, "channel", v.Channel)
			case error:
				slog.Error("receive from channel", "err", v, "channel", chans)
				return
			}
		}
	}
	go func() {
		// listens to the rdb channel, creating a new connection if for whatever reason the recv loop fails
		for {
			conn, err := redigo.Dial("tcp", addr)
			if err != nil {
				slog.Error("failed to get conn for pubsub", "err", err)
			} else {
				recvLoop(conn)
			}
			<-time.After(time.Second)
		}
	}()
	return connCh
}

type LocalBroadcasters struct {
	CountsCaster *UniCaster
	GamesCaster  *MultiCasterMap
	UsersCaster  *MultiCasterMap
}

func MakeBroadcasters() LocalBroadcasters {
	return LocalBroadcasters{
		CountsCaster: MakeUniCaster("counts-caster"),
		GamesCaster:  MakeMultiCasterMap("games-caster", GameExpireDur),
		UsersCaster:  MakeMultiCasterMap("users-caster", -1),
	}
}

func (b *LocalBroadcasters) ListenGameMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.GamesChannel}, func(v redigo.Message) {
		var outputID pb.GameOutputID
		if err := proto.Unmarshal(v.Data, &outputID); err != nil {
			slog.Error("unmarshal game message", "err", err, "channel", v.Channel)
			return
		}
		slog.Info("received message on channel", "key", outputID.GameId, "channel", v.Channel)
		go b.GamesCaster.Broadcast(outputID.GameId, v.Data)
	})
}

func (b *LocalBroadcasters) ListenUsersMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.UsersChannel}, func(v redigo.Message) {
		var userMessage pb.UserMessage
		if err := proto.Unmarshal(v.Data, &userMessage); err != nil {
			slog.Error("unmarshal user message", "err", err, "channel", v.Channel)
			return
		}
		slog.Info("received message on channel", "user", &userMessage, "channel", v.Channel)

		bytes, err := MarshalUserMessageJson(&userMessage)
		if err != nil {
			slog.Error("marshal user message", "err", err, "channel", v.Channel)
			return
		}
		b.UsersCaster.Broadcast(strconv.Itoa(int(userMessage.UserId)), bytes)
	})
}

func (b *LocalBroadcasters) ListenUnicastEvents(rdb db.Redis) chan struct{} {
	var eventMap = map[string]UcEventKind{
		rdb.ActiveCountChannel: UcActiveEk,
		rdb.GamesCountChannel:  UcGamesEk,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(rdb.PubsubAddr, channels, func(v redigo.Message) {
		strData := string(v.Data)
		// unicast broadcasting is only being used to game counts (small data), so it is safe to log
		slog.Info("received event on channel", "event", strData, "channel", v.Channel)

		eKind, ok := eventMap[v.Channel]
		if !ok {
			slog.Error("received event on unmapped channel", "channel", v.Channel, "eventMap", eventMap)
			return
		}
		b.CountsCaster.Broadcast(UcEvent{Kind: eKind, Data: strData})
	})
}

func (svc *Services) BroadcastMessage(ctx context.Context, channel string, b []byte) error {
	conn := svc.Redis.PubSub.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, b); err != nil {
		return fmt.Errorf("publish message to channel %s, %w", channel, err)
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel, "bytesCount", len(b))
	return nil
}

type CountEvent struct {
	Count int64 `json:"count"`
}

func (svc *Services) BroadcastCountEvent(ctx context.Context, channel string, count int64) error {
	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		return fmt.Errorf("marshal count event message: %w", err)
	}
	return svc.BroadcastMessage(ctx, channel, bytes)
}

func (svc *Services) BroadcastActiveCount(ctx context.Context, count int64) error {
	return svc.BroadcastCountEvent(ctx, svc.Redis.ActiveCountChannel, count)
}

func (svc *Services) BroadcastGameCount(ctx context.Context, count int64) error {
	return svc.BroadcastCountEvent(ctx, svc.Redis.GamesCountChannel, count)
}

func (svc *Services) BroadcastGamesEvent(ctx context.Context, message proto.Message) error {
	bytes, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal game event message: %w", err)
	}
	return svc.BroadcastMessage(ctx, svc.Redis.GamesChannel, bytes)
}

func (svc *Services) BroadcastChallenge(ctx context.Context, c ChallengeDTO) error {
	userMessage := SerializeChallengeMessage(c)
	bytes, err := proto.Marshal(userMessage)
	if err != nil {
		return fmt.Errorf("marshal user challenge message: %w", err)
	}
	return svc.BroadcastMessage(ctx, svc.Redis.UsersChannel, bytes)
}
