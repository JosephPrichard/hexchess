package subscribers

import (
	"context"
	"encoding/json/v2"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/challenges"
	"hexchess-svc/gameid"
	"hexchess-svc/pb"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type LocalBroadcasters struct {
	Counts     *GlobalCasterBroker
	Games      *BroadcastBroker[gameid.Identifier]
	Users      *BroadcastBroker[string]
	Tournament *BroadcastBroker[string]
}

func NewLocalBroadcasters() *LocalBroadcasters {
	return &LocalBroadcasters{
		Counts:     NewGlobalCasterBroker("counts-caster"),
		Games:      NewBroadcastBroker[gameid.Identifier]("games-caster"),
		Users:      NewBroadcastBroker[string]("users-caster"),
		Tournament: NewBroadcastBroker[string]("users-caster"),
	}
}

func (b *LocalBroadcasters) Listen(ctx context.Context, rdb cache.Redis) {
	slog.Info("starting local broadcasters")

	var errs []error
	for _, listener := range []func(context.Context, cache.Redis) error{
		b.ListenGameMessages,
		b.ListenUsersMessages,
		b.ListenTournamentMessages,
		b.ListenCountEvents,
	} {
		errs = append(errs, listener(ctx, rdb))
	}

	if e := errors.Join(errs...); e != nil {
		slogutil.Fatal("failed to listen to local broadcasters", e)
	}
}

//func (b *LocalBroadcasters) Shutdown() {
//	slog.Info("shutting down local broadcasters")
//	b.Counts.Shutdown()
//	b.Games.Shutdown()
//	b.Users.Shutdown()
//	b.Tournament.Shutdown()
//}

func (b *LocalBroadcasters) ListenGameMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(ctx, rdb.PubSubClient, []string{cache.Constants.GamesChannel}, func(redisMessage *redis.Message) {
		bytesPayload := []byte(redisMessage.Payload)

		var outputID pb.GameOutputID
		if err := outputID.UnmarshalVT(bytesPayload); err != nil {
			slog.Error("unmarshal game message", "error", err)
			return
		}

		type payload = struct {
			Id string `json:"gameId"`
		}
		slog.Info("received message on channel", "channel", redisMessage.Channel, "payload", payload{outputID.GameId})

		gameID := gameid.Identifier(outputID.GameId)

		b.Games.Broadcast(gameID, bytesPayload)
	})
}

type TournamentOutputKey struct {
	TournamentKey string `json:"tournamentKey"`
}

func (b *LocalBroadcasters) ListenTournamentMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(ctx, rdb.PubSubClient, []string{cache.Constants.TournamentsChannel}, func(redisMessage *redis.Message) {
		bytesPayload := []byte(redisMessage.Payload)

		var output TournamentOutputKey
		if err := json.Unmarshal(bytesPayload, &output); err != nil {
			slog.Error("unmarshal tournament message", "error", err)
			return
		}
		slog.Info("received message on channel", "channel", redisMessage.Channel, "payload", &output)

		b.Tournament.Broadcast(output.TournamentKey, bytesPayload)
	})
}

func (b *LocalBroadcasters) ListenUsersMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(ctx, rdb.PubSubClient, []string{cache.Constants.UsersChannel}, func(redisMessage *redis.Message) {
		bytesPayload := []byte(redisMessage.Payload)

		var userMessage challenges.UserMessage
		if err := json.Unmarshal(bytesPayload, &userMessage); err != nil {
			slog.Error("unmarshal user message", "error", err)
			return
		}
		slog.Info("received message on channel", "channel", redisMessage.Channel, "payload", &redisMessage)

		switch userMessage.Kind {
		case challenges.ChallengeKind:
			b.Users.Broadcast(strconv.Itoa(int(userMessage.Challenge.ChallengeeID)), bytesPayload)
		default:
			slog.Warn("received unknown message kind on users channel", "kind", userMessage.Kind)
		}
	})
}

func (b *LocalBroadcasters) ListenCountEvents(ctx context.Context, rdb cache.Redis) error {
	var eventMap = map[string]CountEventKind{
		cache.Constants.ActiveCountChannel: GlobalActiveEvent,
		cache.Constants.GamesCountChannel:  GlobalGamesEvent,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(ctx, rdb.PubSubClient, channels, func(redisMessage *redis.Message) {
		strData := redisMessage.Payload
		// unicast broadcasting is only being used to game counts (small payload), so it is safe to log
		slog.Info("received message on channel", "event", strData, "channel", redisMessage.Channel)

		eKind, ok := eventMap[redisMessage.Channel]
		if !ok {
			slog.Error("received message on unmapped channel", "channel", redisMessage.Channel, "eventMap", eventMap)
			return
		}
		b.Counts.Broadcast(GlobalCastEvent{Kind: eKind, Data: strData})
	})
}

func listenRedisChannels(ctx context.Context, redisClient *redis.Client, channels []string, onMessage func(*redis.Message)) error {
	// subscribe to all channels
	pubsub := redisClient.Subscribe(ctx, channels...)

	// block and receive at least the subscription acknowledgements
	for range len(channels) {
		anyMsg, err := pubsub.Receive(ctx)
		if err != nil {
			return err
		}
		switch msg := anyMsg.(type) {
		case *redis.Subscription:
			slog.Info("received redis pubsub subscription acknowledgement", "msg", msg, "connURL", redisClient.String())
		case *redis.Message:
			onMessage(msg)
		}
	}

	// start the listening event loop in the background - channel will be closed if pubsub is canceled
	go func() {
		for message := range pubsub.Channel() {
			onMessage(message)
		}
		slog.Info("redis subscriber listener shutdown", "channels", channels)
	}()

	return nil
}
