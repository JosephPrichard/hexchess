package subscribers

import (
	"context"
	"encoding/json/v2"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/challenges"
	"hexchess-svc/pb"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type LocalBroadcasters struct {
	Counts     *GlobalBroadcastBroker[GlobalCastEvent]
	Games      *BroadcastBroker[[]byte]
	Users      *BroadcastBroker[[]byte]
	Tournament *BroadcastBroker[[]byte]
}

type CountEventKind int

const (
	GlobalActiveEvent CountEventKind = iota
	GlobalGamesEvent
)

type GlobalCastEvent struct {
	Kind CountEventKind
	Data string
}

func NewLocalBroadcasters(config BrokerConfig) *LocalBroadcasters {
	return &LocalBroadcasters{
		Counts:     NewGlobalBroadcastBroker[GlobalCastEvent]("counts-caster", config),
		Games:      NewBroadcastBroker[[]byte]("games-caster", config),
		Users:      NewBroadcastBroker[[]byte]("users-caster", config),
		Tournament: NewBroadcastBroker[[]byte]("tournament-caster", config),
	}
}

func (b *LocalBroadcasters) Shutdown() {
	b.Counts.Shutdown()
	b.Games.Shutdown()
	b.Users.Shutdown()
	b.Tournament.Shutdown()
}

func (b *LocalBroadcasters) Listen(ctx context.Context, rdb cache.Redis) {
	slog.Info("PubSub: starting local broadcasters")

	var errs []error
	for _, listener := range []func(context.Context, cache.Redis) error{
		b.listenGameMessages,
		b.listenUsersMessages,
		b.listenTournamentMessages,
		b.listenCountEvents,
	} {
		errs = append(errs, listener(ctx, rdb))
	}

	if e := errors.Join(errs...); e != nil {
		slogutil.Fatal("PubSub: failed to listen to local broadcasters", e)
	}
}

func (b *LocalBroadcasters) listenGameMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: rdb.PubSubClient,
		channels:    []string{cache.Constants.GamesChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var outputID pb.GameOutputID
			if err := outputID.UnmarshalVT(bytesPayload); err != nil {
				slog.Error("PubSub: unmarshal game message", "error", err)
				return
			}

			type payload = struct {
				Id string `json:"gameId"`
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", payload{outputID.GameId})

			b.Games.Broadcast(outputID.GameId, bytesPayload)
		},
	})
}

type TournamentOutputKey struct {
	TournamentKey string `json:"tournamentKey"`
}

func (b *LocalBroadcasters) listenTournamentMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: rdb.PubSubClient,
		channels:    []string{cache.Constants.TournamentsChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var output TournamentOutputKey
			if err := json.Unmarshal(bytesPayload, &output); err != nil {
				slog.Error("PubSub: unmarshal tournament message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", &output)

			b.Tournament.Broadcast(output.TournamentKey, bytesPayload)
		},
	})
}

func (b *LocalBroadcasters) listenUsersMessages(ctx context.Context, rdb cache.Redis) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: rdb.PubSubClient,
		channels:    []string{cache.Constants.UsersChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var userMessage challenges.UserMessage
			if err := json.Unmarshal(bytesPayload, &userMessage); err != nil {
				slog.Error("PubSub: unmarshal user message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", &redisMessage)

			switch userMessage.Kind {
			case challenges.ChallengeKind:
				b.Users.Broadcast(strconv.Itoa(int(userMessage.Challenge.ChallengeeID)), bytesPayload)
			default:
				slog.Warn("received unknown message kind on users channel", "kind", userMessage.Kind)
			}
		},
	})
}

func (b *LocalBroadcasters) listenCountEvents(ctx context.Context, rdb cache.Redis) error {
	var eventMap = map[string]CountEventKind{
		cache.Constants.ActiveCountChannel: GlobalActiveEvent,
		cache.Constants.GamesCountChannel:  GlobalGamesEvent,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: rdb.PubSubClient,
		channels:    channels,
		onMessage: func(redisMessage *redis.Message) {
			strData := redisMessage.Payload
			// unicast broadcasting is only being used to game counts (small payload), so it is safe to log
			slog.Info("PubSub: received message on channel", "event", strData, "channel", redisMessage.Channel)

			eKind, ok := eventMap[redisMessage.Channel]
			if !ok {
				slog.Error("PubSub: received message on unmapped channel", "channel", redisMessage.Channel, "eventMap", eventMap)
				return
			}
			b.Counts.Broadcast(GlobalCastEvent{Kind: eKind, Data: strData})
		},
	})
}

type listenConfig struct {
	ctx         context.Context
	redisClient *redis.Client
	channels    []string
	onMessage   func(*redis.Message)
}

func listenRedisChannels(cfg listenConfig) error {
	ctx := cfg.ctx
	redisClient := cfg.redisClient
	channels := cfg.channels
	onMessage := cfg.onMessage

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
			slog.Info("PubSub: received redis pubsub subscription acknowledgement", "msg", msg, "connURL", redisClient.String())
		case *redis.Message:
			onMessage(msg)
		}
	}

	go func() {
		// start the listening event loop - channel will be closed if pubsub is closed, which is closed if redis is closed
		for message := range pubsub.Channel() {
			onMessage(message)
		}
		slog.Info("PubSub: redis subscriber listener shutdown", "channels", channels)
	}()

	return nil
}
