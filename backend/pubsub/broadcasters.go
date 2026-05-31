package pubsub

import (
	"context"
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"
)

//go:generate mockgen -source=broadcasters.go -destination=./broadcasters_mock.go -package=pubsub

type BroadcasterAPI interface {
	BroadcastActiveCount(ctx context.Context, count int64)
	BroadcastGameCount(ctx context.Context, count int64)
	BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput)
	BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput)
	BroadcastChallenge(ctx context.Context, challenge model.Challenge)
}

type NoopBroadcaster struct{}

func (n *NoopBroadcaster) BroadcastActiveCount(ctx context.Context, count int64) error { return nil }
func (n *NoopBroadcaster) BroadcastGameCount(ctx context.Context, count int64) error   { return nil }
func (n *NoopBroadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) error {
	return nil
}
func (n *NoopBroadcaster) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) error {
	return nil
}
func (n *NoopBroadcaster) BroadcastChallenge(ctx context.Context, challenge model.Challenge) error {
	return nil
}

type Broadcaster struct {
	redis *redis.Pool
	names db.RedisNames
}

func MakeBroadcaster(redis db.Redis) *Broadcaster {
	return &Broadcaster{redis: redis.PubSub, names: redis.RedisNames}
}

func (svc *Broadcaster) broadcastMessage(ctx context.Context, channel string, bytes []byte) {
	conn := svc.redis.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, bytes); err != nil {
		slog.ErrorContext(ctx, "failed to publish message to channel", "channel", channel, "error", err)
	} else {
		slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel)
	}
}

type CountEvent struct {
	Count int64 `json:"count"`
}

func (svc *Broadcaster) broadcastCountEvent(ctx context.Context, channel string, count int64) {
	slog.InfoContext(ctx, "broadcasting count event", "channel", channel, "count", count)

	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal count event", "error", err)
		return
	}
	go svc.broadcastMessage(context.WithoutCancel(ctx), channel, bytes)
}

func (svc *Broadcaster) BroadcastActiveCount(ctx context.Context, count int64) {
	svc.broadcastCountEvent(ctx, svc.names.ActiveCountChannel, count)
}

func (svc *Broadcaster) BroadcastGameCount(ctx context.Context, count int64) {
	svc.broadcastCountEvent(ctx, svc.names.GamesCountChannel, count)
}

func (svc *Broadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) {
	slog.InfoContext(ctx, "broadcasting game event", "gameOutput", output)

	bytes, err := proto.Marshal(output)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal game message", "error", err)
		return
	}
	go svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.GamesChannel, bytes)
}

func (svc *Broadcaster) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) {
	slog.InfoContext(ctx, "broadcasting tournament", "tournamentOutput", tournament)

	bytes, err := proto.Marshal(tournament)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal tournament message", "error", err)
		return
	}
	go svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.TournamentsChannel, bytes)
}

func (svc *Broadcaster) BroadcastChallenge(ctx context.Context, challenge model.Challenge) {
	slog.InfoContext(ctx, "broadcasting challenge", "challenge", challenge)

	bytes, err := proto.Marshal(model.SerializeChallengeMessage(challenge))
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal user challenge message", "error", err)
		return
	}
	go svc.broadcastMessage(context.WithoutCancel(ctx), svc.names.UsersChannel, bytes)
}
