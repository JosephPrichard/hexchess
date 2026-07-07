package pubsub

import (
	"context"
	"encoding/json"
	"hexchess-lib/async"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
)

type Broadcaster struct {
	redis      *redis.Pool
	names      db.RedisNames
	dispatcher async.Dispatcher
}

func NewSyncBroadcaster(redis db.Redis) *Broadcaster {
	return &Broadcaster{
		redis:      redis.PubSub,
		names:      redis.RedisNames,
		dispatcher: async.SyncDispatcher{},
	}
}

func NewAsyncBroadcaster(redis db.Redis) *Broadcaster {
	return &Broadcaster{
		redis:      redis.PubSub,
		names:      redis.RedisNames,
		dispatcher: async.AsyncDispatcher{},
	}
}

func (b *Broadcaster) broadcastMessage(ctx context.Context, channel string, bytes []byte) {
	b.dispatcher.Go(func() {
		conn := b.redis.Get()
		defer conn.Close()

		if _, err := conn.Do("PUBLISH", channel, bytes); err != nil {
			slog.ErrorContext(ctx, "failed to publish message to channel", "channel", channel, "error", err)
		} else {
			slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel)
		}
	})
}

type CountEvent struct {
	Count int64 `json:"count"`
}

func (b *Broadcaster) broadcastCountEvent(ctx context.Context, channel string, count int64) {
	slog.InfoContext(ctx, "broadcasting count event", "channel", channel, "count", count)

	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal count event", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), channel, bytes)
}

func (b *Broadcaster) BroadcastActiveCount(ctx context.Context, count int64) {
	b.broadcastCountEvent(ctx, b.names.ActiveCountChannel, count)
}

func (b *Broadcaster) BroadcastGameCount(ctx context.Context, count int64) {
	b.broadcastCountEvent(ctx, b.names.GamesCountChannel, count)
}

func (b *Broadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) {
	slog.InfoContext(ctx, "broadcasting game event", "gameId", output.GameId)

	bytes, err := proto.Marshal(output)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal game message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), b.names.GamesChannel, bytes)
}

func (b *Broadcaster) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) {
	slog.InfoContext(ctx, "broadcasting tournament", "tournamentOutput", tournament)

	bytes, err := proto.Marshal(tournament)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal tournament message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), b.names.TournamentsChannel, bytes)
}

func (b *Broadcaster) BroadcastChallenge(ctx context.Context, challenge model.Challenge) {
	slog.InfoContext(ctx, "broadcasting challenge", "challenge", challenge)

	bytes, err := proto.Marshal(model.SerializeChallengeMessage(challenge))
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal user challenge message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), b.names.UsersChannel, bytes)
}
