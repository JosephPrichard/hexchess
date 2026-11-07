package publishers

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/cache"
	"hexchess-svc/pb"
	"hexchess-svc/utils/async"
	"log/slog"

	"github.com/gomodule/redigo/redis"
)

type Broadcaster struct {
	redis      *redis.Pool
	dispatcher async.Dispatcher
}

func NewSyncBroadcaster(redis cache.Redis) Broadcaster {
	return Broadcaster{
		redis:      redis.PubSubClient,
		dispatcher: async.SyncDispatcher{},
	}
}

func NewAsyncBroadcaster(redis cache.Redis) Broadcaster {
	return Broadcaster{
		redis:      redis.PubSubClient,
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
	b.broadcastCountEvent(ctx, cache.Constants.ActiveCountChannel, count)
}

func (b *Broadcaster) BroadcastGameCount(ctx context.Context, count int64) {
	b.broadcastCountEvent(ctx, cache.Constants.GamesCountChannel, count)
}

func (b *Broadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) {
	slog.InfoContext(ctx, "broadcasting game event", "gameId", output.GameId)

	bytes, err := output.MarshalVT()
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal game message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), cache.Constants.GamesChannel, bytes)
}

func (b *Broadcaster) BroadcastTournament(ctx context.Context, tournament any) {
	slog.InfoContext(ctx, "broadcasting tournament", "tournamentOutput", tournament)

	bytes, err := json.Marshal(tournament)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal tournament message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), cache.Constants.TournamentsChannel, bytes)
}

func (b *Broadcaster) BroadcastUserMessage(ctx context.Context, message any) {
	slog.InfoContext(ctx, "broadcasting user message", "message", message)

	bytes, err := json.Marshal(message)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal user message", "error", err)
		return
	}
	b.broadcastMessage(context.WithoutCancel(ctx), cache.Constants.UsersChannel, bytes)
}
