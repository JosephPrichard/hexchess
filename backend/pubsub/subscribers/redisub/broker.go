package redisub

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"hexchess-svc/challenges"
	"hexchess-svc/pb"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisBroadcastBroker struct {
	client     *redis.Client
	Counts     *GlobalBroadcaster
	Games      *TopicBroadcaster
	Users      *TopicBroadcaster
	Tournament *TopicBroadcaster
}

func NewRedisBroadcastBroker(config BrokerConfig, client *redis.Client) *RedisBroadcastBroker {
	if client == nil {
		panic("PubSub: while initializing broadcast broker, redis client was nil")
	}
	return &RedisBroadcastBroker{
		client:     client,
		Counts:     NewGlobalBroadcaster("counts-caster", config),
		Games:      NewTopicBroadcaster("games-caster", config),
		Users:      NewTopicBroadcaster("users-caster", config),
		Tournament: NewTopicBroadcaster("tournament-caster", config),
	}
}

func (b *RedisBroadcastBroker) Close() {
	b.Counts.Close()
	b.Games.Close()
	b.Users.Close()
	b.Tournament.Close()
}

func (b *RedisBroadcastBroker) Subscribe(ctx context.Context, channel string, topic string) (c <-chan []byte, f func(), e error) {
	switch channel {
	case event.GamesChannel:
		c, f = b.Games.Subscribe(topic, event.ChannelBufSize)
	case event.UsersChannel:
		c, f = b.Users.Subscribe(topic, event.ChannelBufSize)
	case event.TournamentsChannel:
		c, f = b.Tournament.Subscribe(topic, event.ChannelBufSize)
	case event.ActiveCountChannel, event.GamesCountChannel:
		c, f = b.Counts.Subscribe(event.ChannelBufSize)
	default:
		slog.ErrorContext(ctx, "Pubsub: cannot subscribe to unimplemented channel", "channel", channel)
		e = fmt.Errorf("cannot subscribe to unimplemented channel: %s", channel)
	}
	return c, f, e
}

func (b *RedisBroadcastBroker) Start(ctx context.Context) {
	slog.Info("PubSub: starting local broadcasters")

	var errs []error
	for _, listener := range []func(context.Context, *redis.Client) error{
		b.listenGameMessages,
		b.listenUsersMessages,
		b.listenTournamentMessages,
		b.listenCountEvents,
	} {
		errs = append(errs, listener(ctx, b.client))
	}

	if e := errors.Join(errs...); e != nil {
		slogutil.Fatal("PubSub: failed to listen to local broadcasters", e)
	}
}

func (b *RedisBroadcastBroker) listenGameMessages(ctx context.Context, client *redis.Client) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: client,
		channels:    []string{event.GamesChannel},
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

			b.Games.Publish(outputID.GameId, bytesPayload)
		},
	})
}

type TournamentOutputKey struct {
	TournamentKey string `json:"tournamentKey"`
}

func (b *RedisBroadcastBroker) listenTournamentMessages(ctx context.Context, client *redis.Client) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: client,
		channels:    []string{event.TournamentsChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var output TournamentOutputKey
			if err := json.Unmarshal(bytesPayload, &output); err != nil {
				slog.Error("PubSub: unmarshal tournament message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", &output)

			b.Tournament.Publish(output.TournamentKey, bytesPayload)
		},
	})
}

func (b *RedisBroadcastBroker) listenUsersMessages(ctx context.Context, client *redis.Client) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: client,
		channels:    []string{event.UsersChannel},
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
				b.Users.Publish(strconv.Itoa(int(userMessage.Challenge.ChallengeeID)), bytesPayload)
			default:
				slog.Warn("received unknown message kind on users channel", "kind", userMessage.Kind)
			}
		},
	})
}

func (b *RedisBroadcastBroker) listenCountEvents(ctx context.Context, client *redis.Client) error {
	var eventMap = map[string]event.CountEventKind{
		event.ActiveCountChannel: event.GlobalActiveEvent,
		event.GamesCountChannel:  event.GlobalGamesEvent,
	}

	var channels []string
	for ch := range eventMap {
		channels = append(channels, ch)
	}

	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: client,
		channels:    channels,
		onMessage: func(redisMessage *redis.Message) {
			strData := redisMessage.Payload
			// global broadcasting is only being used to broadcast counts (small payload), so it is safe to log
			slog.Info("PubSub: received message on channel", "event", strData, "channel", redisMessage.Channel)

			eKind, ok := eventMap[redisMessage.Channel]
			if !ok {
				slog.Error("PubSub: received message on unmapped channel", "channel", redisMessage.Channel, "eventMap", eventMap)
				return
			}

			var countEvent event.CountEvent
			if err := json.Unmarshal([]byte(strData), &countEvent); err != nil {
				slog.Error("PubSub: failed to unmarshal countEvent", "error", err)
				return
			}
			countEvent.Kind = eKind

			payload, err := json.Marshal(countEvent)
			if err != nil {
				slog.Error("PubSub: failed to marshal countEvent event", "error", err)
				return
			}

			b.Counts.Broadcast(payload)
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
