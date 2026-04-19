package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/domain"
	"hexchess-svc/pb"
	"log/slog"
	"strconv"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
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
	CountsCaster     *UniCaster
	GamesCaster      *MultiCasterMap
	UsersCaster      *MultiCasterMap
	TournamentCaster *MultiCasterMap
}

func MakeLocalBroadcasters() *LocalBroadcasters {
	return &LocalBroadcasters{
		CountsCaster:     MakeUniCaster("counts-caster"),
		GamesCaster:      MakeMultiCasterMap("games-caster", -1),
		UsersCaster:      MakeMultiCasterMap("users-caster", -1),
		TournamentCaster: MakeMultiCasterMap("users-caster", -1),
	}
}

func (b *LocalBroadcasters) Listen(rdb db.Redis) {
	<-b.ListenGameMessages(rdb)
	<-b.ListenUsersMessages(rdb)
	<-b.ListenUnicastEvents(rdb)
}

func (b *LocalBroadcasters) ListenGameMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.GamesChannel}, func(v redigo.Message) {
		var outputID pb.GameOutputID
		if err := proto.Unmarshal(v.Data, &outputID); err != nil {
			slog.Error("unmarshal game message", "err", err)
			return
		}

		slog.Info("received message on games channel", "key", outputID.GameId)
		go b.GamesCaster.Broadcast(outputID.GameId, v.Data)
	})
}

func (b *LocalBroadcasters) ListenTournamentMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.TournamentsChannel}, func(v redigo.Message) {
		var output pb.TournamentOutput
		if err := proto.Unmarshal(v.Data, &output); err != nil {
			slog.Error("unmarshal tournament message", "err", err)
			return
		}

		bytes, err := MarshalTournamentOutputJson(&output)
		if err != nil {
			slog.Error("failed to transition tournament output event to json", "err", err)
			return
		}

		slog.Info("received message on tournaments channel", "key", output.TournamentKey)
		go b.TournamentCaster.Broadcast(output.TournamentKey, bytes)
	})
}

func (b *LocalBroadcasters) ListenUsersMessages(rdb db.Redis) chan struct{} {
	return listenRedisChannels(rdb.PubsubAddr, []string{rdb.UsersChannel}, func(v redigo.Message) {
		var userMessage pb.UserMessage
		if err := proto.Unmarshal(v.Data, &userMessage); err != nil {
			slog.Error("unmarshal user message", "err", err)
			return
		}
		slog.Info("received message on channel", "user", &userMessage)

		bytes, err := MarshalUserMessageJson(&userMessage)
		if err != nil {
			slog.Error("marshal user message", "err", err)
			return
		}

		go b.UsersCaster.Broadcast(strconv.Itoa(int(userMessage.UserId)), bytes)
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

func (svc *HexchessServices) BroadcastMessage(ctx context.Context, channel string, b []byte) error {
	conn := svc.redis.PubSub.Get()
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

func (svc *HexchessServices) BroadcastCountEvent(ctx context.Context, channel string, count int64) error {
	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		return fmt.Errorf("marshal count event message: %w", err)
	}
	return svc.BroadcastMessage(ctx, channel, bytes)
}

func (svc *HexchessServices) BroadcastActiveCount(ctx context.Context, count int64) error {
	return svc.BroadcastCountEvent(ctx, svc.redis.ActiveCountChannel, count)
}

func (svc *HexchessServices) BroadcastGameCount(ctx context.Context, count int64) error {
	return svc.BroadcastCountEvent(ctx, svc.redis.GamesCountChannel, count)
}

func (svc *HexchessServices) BroadcastGamesEvent(ctx context.Context, message proto.Message) error {
	bytes, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal game event message: %w", err)
	}
	return svc.BroadcastMessage(ctx, svc.redis.GamesChannel, bytes)
}

func (svc *HexchessServices) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) error {
	bytes, err := proto.Marshal(tournament)
	if err != nil {
		return fmt.Errorf("marshal tournament message: %w", err)
	}
	return svc.BroadcastMessage(ctx, svc.redis.TournamentsChannel, bytes)
}

func (svc *HexchessServices) BroadcastChallenge(ctx context.Context, challenge domain.Challenge) error {
	userMessage := SerializeChallengeMessage(challenge)

	bytes, err := proto.Marshal(userMessage)
	if err != nil {
		return fmt.Errorf("marshal user challenge message: %w", err)
	}

	return svc.BroadcastMessage(ctx, svc.redis.UsersChannel, bytes)
}

func (svc *HexchessServices) BroadcastTournamentParticipant(ctx context.Context, tournamentKey uuid.UUID, userID int64, mode domain.GameMode) error {
	lbdUser, err := svc.GetLeaderboardUser(ctx, userID, mode)
	if err != nil {
		return fmt.Errorf("get leaderboard user by user id %d: %w", userID, err)
	}

	tournament := SerializeParticipantOutput(tournamentKey, lbdUser)
	if err := svc.BroadcastTournament(ctx, tournament); err != nil {
		return fmt.Errorf("broadcast tournament participant: %w", err)
	}

	slog.InfoContext(ctx, "broadcasted selected tournament participant", "lbdUser", lbdUser)
	return nil
}

func (svc *HexchessServices) BroadcastStartTournamentCountdown(ctx context.Context, tournamentKey uuid.UUID) error {
	tournament := SerializeBeginTournamentCountdown(tournamentKey)

	if err := svc.BroadcastTournament(ctx, tournament); err != nil {
		return fmt.Errorf("broadcast tournament participant: %w", err)
	}

	slog.InfoContext(ctx, "broadcasted start of tournament countdown", "tournamentKey", tournamentKey)
	return nil
}
