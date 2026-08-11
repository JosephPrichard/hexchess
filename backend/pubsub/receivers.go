package pubsub

import (
	"hexchess-svc/cache"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	redigo "github.com/gomodule/redigo/redis"
)

func listenRedisChannels(addr string, chans []string, onMessage func(m redigo.Message)) chan struct{} {
	connCh := make(chan struct{}, 1) // send a signal whenever the connection is complete

	recvLoop := func(conn redigo.Conn) {
		psc := redigo.PubSubConn{Conn: conn}
		defer psc.Close()

		for _, channel := range chans {
			err := psc.Subscribe(channel)
			if err != nil {
				slog.Error("receive from channel", "error", err, "channel", channel)
				return
			}
		}
		slog.Info("listening redis pubsub channel subscriber", "channels", chans)

		// signals to the caller whenever the background routine is *actually* listening on the channels
		connCh <- struct{}{}
		for {
			switch v := psc.Receive().(type) {
			case redigo.Message:
				onMessage(v)
			case redigo.Subscription:
				slog.Info("received subscription on channel", "kind", v.Kind, "channel", v.Channel)
			case error:
				slog.Error("receive from channel", "error", v.Error(), "channel", chans)
				return
			}
		}
	}

	go func() {
		// listens to the redis channel, creating a new connection if the recv loop fails
		for {
			conn, err := redigo.Dial("tcp", addr)
			if err != nil {
				slog.Error("failed to get conn for pubsub", "addr", addr, "error", err)
			} else {
				recvLoop(conn)
			}
			<-time.After(time.Second)
		}
	}()
	return connCh
}

type LocalBroadcasters struct {
	Counts     *GlobalCasterBroker
	Games      *BroadcastBroker[model.GameID]
	Users      *BroadcastBroker[string]
	Tournament *BroadcastBroker[string]
}

func NewLocalBroadcasters() *LocalBroadcasters {
	return &LocalBroadcasters{
		Counts:     NewGlobalCasterBroker("counts-caster"),
		Games:      NewBroadcastBroker[model.GameID]("games-caster"),
		Users:      NewBroadcastBroker[string]("users-caster"),
		Tournament: NewBroadcastBroker[string]("users-caster"),
	}
}

func (b *LocalBroadcasters) Listen(rdb cache.Redis) {
	slog.Info("starting local broadcasters")

	var chans []chan struct{}
	for _, listener := range []func(db cache.Redis) chan struct{}{
		b.ListenGameMessages,
		b.ListenUsersMessages,
		b.ListenTournamentMessages,
		b.ListenCountEvents,
	} {
		chans = append(chans, listener(rdb))
	}
	for _, ch := range chans {
		<-ch
	}
}

//func (b *LocalBroadcasters) Shutdown() {
//	slog.Info("shutting down local broadcasters")
//	b.Counts.Shutdown()
//	b.Games.Shutdown()
//	b.Users.Shutdown()
//	b.Tournament.Shutdown()
//}

func (b *LocalBroadcasters) ListenGameMessages(rdb cache.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{cache.Constants.GamesChannel}, func(v redigo.Message) {
		var outputID pb.GameOutputID
		if err := outputID.UnmarshalVT(v.Data); err != nil {
			slog.Error("unmarshal game message", "error", err)
			return
		}

		type payload = struct {
			Id string `json:"gameId"`
		}
		slog.Info("received message on channel", "channel", cache.Constants.GamesChannel, "payload", payload{outputID.GameId})

		gameID := model.GameID(outputID.GameId)

		b.Games.Broadcast(gameID, v.Data)
	})
}

func (b *LocalBroadcasters) ListenTournamentMessages(rdb cache.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{cache.Constants.TournamentsChannel}, func(v redigo.Message) {
		var output model.TournamentOutputKey
		if err := sonic.Unmarshal(v.Data, &output); err != nil {
			slog.Error("unmarshal tournament message", "error", err)
			return
		}
		slog.Info("received message on channel", "channel", cache.Constants.TournamentsChannel, "payload", &output)

		b.Tournament.Broadcast(output.TournamentKey, v.Data)
	})
}

func (b *LocalBroadcasters) ListenUsersMessages(rdb cache.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{cache.Constants.UsersChannel}, func(v redigo.Message) {
		var message model.UserMessage
		if err := sonic.Unmarshal(v.Data, &message); err != nil {
			slog.Error("unmarshal user message", "error", err)
			return
		}
		slog.Info("received message on channel", "channel", cache.Constants.UsersChannel, "payload", &message)

		switch message.Kind {
		case model.ChallengeKind:
			b.Users.Broadcast(strconv.Itoa(int(message.Challenge.ChallengeeID)), v.Data)
		default:
			slog.Warn("received unknown message kind on users channel", "kind", message.Kind)
		}
	})
}

func (b *LocalBroadcasters) ListenCountEvents(rdb cache.Redis) chan struct{} {
	var eventMap = map[string]CountEventKind{
		cache.Constants.ActiveCountChannel: GlobalActiveEvent,
		cache.Constants.GamesCountChannel:  GlobalGamesEvent,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(rdb.PubsubAddr, channels, func(v redigo.Message) {
		strData := string(v.Data)
		// unicast broadcasting is only being used to game counts (small payload), so it is safe to log
		slog.Info("received message on channel", "event", strData, "channel", v.Channel)

		eKind, ok := eventMap[v.Channel]
		if !ok {
			slog.Error("received message on unmapped channel", "channel", v.Channel, "eventMap", eventMap)
			return
		}
		b.Counts.Broadcast(GlobalCastEvent{Kind: eKind, Data: strData})
	})
}
