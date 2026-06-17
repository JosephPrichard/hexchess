package pubsub

import (
	"hexchess-svc/db"
	"hexchess-svc/model"
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
		slog.Info("starting redis pubsub channel subscriber", "channels", chans)

		// signals to the caller whenever the background routine is *actually* listening on the channels
		connCh <- struct{}{}
		for {
			switch v := psc.Receive().(type) {
			case redigo.Message:
				onMessage(v)
			case redigo.Subscription:
				slog.Info("received subscription on channel", "value", v, "channel", v.Channel)
			case error:
				slog.Error("receive from channel", "error", v, "channel", chans)
				return
			}
		}
	}
	go func() {
		// listens to the rdb channel, creating a new connection if for whatever reason the recv loop fails
		for {
			conn, err := redigo.Dial("tcp", addr)
			if err != nil {
				slog.Error("failed to get conn for pubsub", "error", err)
			} else {
				recvLoop(conn)
			}
			<-time.After(time.Second)
		}
	}()
	return connCh
}

type LocalBroadcasters struct {
	CountsCaster     *GlobalCasterActor
	GamesCaster      *BroadcastActor[model.GameID]
	UsersCaster      *BroadcastActor[string]
	TournamentCaster *BroadcastActor[string]
}

func MakeLocalBroadcasters() *LocalBroadcasters {
	return &LocalBroadcasters{
		CountsCaster:     MakeGlobalCasterActor("counts-caster"),
		GamesCaster:      MakeBroadcastActor[model.GameID]("games-caster"),
		UsersCaster:      MakeBroadcastActor[string]("users-caster"),
		TournamentCaster: MakeBroadcastActor[string]("users-caster"),
	}
}

func (b *LocalBroadcasters) Listen(rdb db.Redis) {
	slog.Info("starting local broadcasters")

	var chans []chan struct{}
	for _, listener := range []func(db db.Redis) chan struct{}{
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

func (b *LocalBroadcasters) Shutdown() {
	slog.Info("shutting down local broadcasters")
	b.CountsCaster.Shutdown()
}

func (b *LocalBroadcasters) ListenGameMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.GamesChannel}, func(v redigo.Message) {
		var outputID pb.GameOutputID
		if err := proto.Unmarshal(v.Data, &outputID); err != nil {
			slog.Error("unmarshal game message", "error", err)
			return
		}
		slog.Info("received message on games channel", "key", outputID.GameId)

		gameID := model.GameID(outputID.GameId)

		go b.GamesCaster.Broadcast(gameID, v.Data)
	})
}

func (b *LocalBroadcasters) ListenTournamentMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.TournamentsChannel}, func(v redigo.Message) {
		var output pb.TournamentOutput
		if err := proto.Unmarshal(v.Data, &output); err != nil {
			slog.Error("unmarshal tournament message", "error", err)
			return
		}
		slog.Info("received message on tournaments channel", "key", output.TournamentKey, "output", &output)

		bytes, err := model.MarshalTournamentOutputJson(&output)
		if err != nil {
			slog.Error("failed to transition tournament output event to json", "error", err)
			return
		}

		go b.TournamentCaster.Broadcast(output.TournamentKey, bytes)
	})
}

func (b *LocalBroadcasters) ListenUsersMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.UsersChannel}, func(v redigo.Message) {
		var userMessage pb.UserMessage
		if err := proto.Unmarshal(v.Data, &userMessage); err != nil {
			slog.Error("unmarshal user message", "error", err)
			return
		}
		slog.Info("received message on users channel", "user", &userMessage)

		bytes, err := model.MarshalUserMessageJson(&userMessage)
		if err != nil {
			slog.Error("marshal user message", "error", err)
			return
		}

		go b.UsersCaster.Broadcast(strconv.Itoa(int(userMessage.UserId)), bytes)
	})
}

func (b *LocalBroadcasters) ListenCountEvents(rdb db.Redis) chan struct{} {
	var eventMap = map[string]CountEventKind{
		rdb.ActiveCountChannel: GlobalActiveEvent,
		rdb.GamesCountChannel:  GlobalGamesEvent,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(rdb.PubsubAddr, channels, func(v redigo.Message) {
		strData := string(v.Data)
		// unicast broadcasting is only being used to game counts (small payload), so it is safe to log
		slog.Info("received event on channel", "event", strData, "channel", v.Channel)

		eKind, ok := eventMap[v.Channel]
		if !ok {
			slog.Error("received event on unmapped channel", "channel", v.Channel, "eventMap", eventMap)
			return
		}
		b.CountsCaster.Broadcast(GlobalCastEvent{Kind: eKind, Data: strData})
	})
}
