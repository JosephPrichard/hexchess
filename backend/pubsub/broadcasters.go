package pubsub

import (
	"context"
	"encoding/json"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
)

type BroadcastOptions struct {
	isAsync bool
}

type BroadcastOption func(*BroadcastOptions)

func Async() BroadcastOption {
	return func(opts *BroadcastOptions) {
		opts.isAsync = true
	}
}

func Sync() BroadcastOption {
	return func(opts *BroadcastOptions) {
		opts.isAsync = false
	}
}

type Broadcaster struct {
	redis *redis.Pool
	names db.RedisNames
}

func NewBroadcaster(redis db.Redis) *Broadcaster {
	return &Broadcaster{redis: redis.PubSub, names: redis.RedisNames}
}

func (svc *Broadcaster) broadcastMessage(ctx context.Context, channel string, bytes []byte, opts ...BroadcastOption) {
	var broadcastOpts BroadcastOptions
	for _, opt := range opts {
		opt(&broadcastOpts)
	}

	exec := func() {
		conn := svc.redis.Get()
		defer conn.Close()

		if _, err := conn.Do("PUBLISH", channel, bytes); err != nil {
			slog.ErrorContext(ctx, "failed to publish message to channel", "channel", channel, "error", err)
		} else {
			slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel)
		}
	}
	if broadcastOpts.isAsync {
		go exec()
	} else {
		exec()
	}
}

type CountEvent struct {
	Count int64 `json:"count"`
}

func (svc *Broadcaster) broadcastCountEvent(ctx context.Context, channel string, count int64, opts ...BroadcastOption) {
	slog.InfoContext(ctx, "broadcasting count event", "channel", channel, "count", count)

	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal count event", "error", err)
		return
	}
	svc.broadcastMessage(context.WithoutCancel(ctx), channel, bytes, opts...)
}

func (svc *Broadcaster) BroadcastActiveCount(ctx context.Context, count int64, opts ...BroadcastOption) {
	svc.broadcastCountEvent(ctx, svc.names.ActiveCountChannel, count, opts...)
}

func (svc *Broadcaster) BroadcastGameCount(ctx context.Context, count int64, opts ...BroadcastOption) {
	svc.broadcastCountEvent(ctx, svc.names.GamesCountChannel, count, opts...)
}

func (svc *Broadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput, opts ...BroadcastOption) {
	slog.InfoContext(ctx, "broadcasting game event", "gameOutput", output)

	bytes, err := proto.Marshal(output)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal game message", "error", err)
		return
	}
	svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.GamesChannel, bytes, opts...)
}

func (svc *Broadcaster) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput, opts ...BroadcastOption) {
	slog.InfoContext(ctx, "broadcasting tournament", "tournamentOutput", tournament)

	bytes, err := proto.Marshal(tournament)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal tournament message", "error", err)
		return
	}
	svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.TournamentsChannel, bytes, opts...)
}

func (svc *Broadcaster) BroadcastChallenge(ctx context.Context, challenge model.Challenge, opts ...BroadcastOption) {
	slog.InfoContext(ctx, "broadcasting challenge", "challenge", challenge)

	bytes, err := proto.Marshal(model.SerializeChallengeMessage(challenge))
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal user challenge message", "error", err)
		return
	}
	svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.UsersChannel, bytes, opts...)
}
