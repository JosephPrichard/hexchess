package redisub

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"hexchess-svc/proto/pb"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisBroadcastBroker struct {
	client      *redis.Client
	ActiveCount *GlobalBroadcaster
	GameCount   *GlobalBroadcaster
	Games       *TopicBroadcaster
	Users       *TopicBroadcaster
	Tournament  *TopicBroadcaster
}

func NewRedisBroadcastBroker(config BrokerConfig, client *redis.Client) *RedisBroadcastBroker {
	if client == nil {
		panic("PubSub: while initializing broadcast broker, redis client was nil")
	}
	return &RedisBroadcastBroker{
		client:      client,
		ActiveCount: NewGlobalBroadcaster("user-counts-broker", config),
		GameCount:   NewGlobalBroadcaster("game-counts-broker", config),
		Games:       NewTopicBroadcaster("games-broker", config),
		Users:       NewTopicBroadcaster("users-broker", config),
		Tournament:  NewTopicBroadcaster("tournament-broker", config),
	}
}

func (b *RedisBroadcastBroker) Close() {
	b.ActiveCount.Close()
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
	case event.ActiveCountChannel:
		c, f = b.ActiveCount.Subscribe(event.ChannelBufSize)
	case event.GamesCountChannel:
		c, f = b.GameCount.Subscribe(event.ChannelBufSize)
	default:
		slog.ErrorContext(ctx, "PubSub: cannot subscribe to unimplemented channel", "channel", channel)
		e = fmt.Errorf("cannot subscribe to unimplemented channel: %s", channel)
	}
	return c, f, e
}

func (b *RedisBroadcastBroker) Start(ctx context.Context) {
	slog.Info("PubSub: starting local broadcasters")

	var errs []error
	for _, listener := range []func(context.Context) error{
		b.listenGameMessages,
		b.listenUsersMessages,
		b.listenTournamentMessages,
		b.listenActiveCountMessages,
		b.listenGameCountMessages,
	} {
		errs = append(errs, listener(ctx))
	}

	if e := errors.Join(errs...); e != nil {
		slogutil.Fatal("PubSub: failed to listen to local broadcasters", e)
	}
}

func (b *RedisBroadcastBroker) listenGameMessages(ctx context.Context) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: b.client,
		channels:    []string{event.GamesChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var oID pb.GameOutputID
			if err := oID.UnmarshalVT(bytesPayload); err != nil {
				slog.Error("PubSub: unmarshal game message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "gameID", oID.GameId)

			b.Games.Broadcast(oID.GameId, bytesPayload)
		},
	})
}

type TournamentEventKey struct {
	TournamentKey string `json:"tournamentKey"`
}

func (b *RedisBroadcastBroker) listenTournamentMessages(ctx context.Context) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: b.client,
		channels:    []string{event.TournamentsChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var e TournamentEventKey
			if err := json.Unmarshal(bytesPayload, &e); err != nil {
				slog.Error("PubSub: unmarshal tournament message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", &e)

			b.Tournament.Broadcast(e.TournamentKey, bytesPayload)
		},
	})
}

type UserEventKey struct {
	UserID int64 `json:"userId"`
}

func (b *RedisBroadcastBroker) listenUsersMessages(ctx context.Context) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: b.client,
		channels:    []string{event.UsersChannel},
		onMessage: func(redisMessage *redis.Message) {
			bytesPayload := []byte(redisMessage.Payload)

			var e UserEventKey
			if err := json.Unmarshal(bytesPayload, &e); err != nil {
				slog.Error("PubSub: unmarshal user message", "error", err)
				return
			}
			slog.Info("PubSub: received message on channel", "channel", redisMessage.Channel, "payload", &redisMessage)

			userID := strconv.Itoa(int(e.UserID))
			b.Users.Broadcast(userID, bytesPayload)
		},
	})
}

func (b *RedisBroadcastBroker) listenActiveCountMessages(ctx context.Context) error {
	return b.listenCountMessages(ctx, event.ActiveCountChannel, b.ActiveCount)
}

func (b *RedisBroadcastBroker) listenGameCountMessages(ctx context.Context) error {
	return b.listenCountMessages(ctx, event.GamesCountChannel, b.GameCount)
}

func (b *RedisBroadcastBroker) listenCountMessages(ctx context.Context, channel string, broadcaster *GlobalBroadcaster) error {
	return listenRedisChannels(listenConfig{
		ctx:         ctx,
		redisClient: b.client,
		channels:    []string{channel},
		onMessage: func(redisMessage *redis.Message) {
			slog.Info("PubSub: received message on channel", "event", redisMessage.Payload, "channel", redisMessage.Channel)
			broadcaster.Broadcast([]byte(redisMessage.Payload))
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
