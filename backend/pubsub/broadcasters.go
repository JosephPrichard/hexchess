package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/db"
	"hexchess-svc/model"
	"hexchess-svc/pb"
	"log/slog"
)

//go:generate mockgen -source=broadcasters.go -destination=./broadcasters_mock.go -package=pubsub

type BroadcasterAPI interface {
	BroadcastActiveCount(ctx context.Context, count int64) error
	BroadcastGameCount(ctx context.Context, count int64) error
	BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) error
	BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) error
	BroadcastChallenge(ctx context.Context, challenge model.Challenge) error
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

func (svc *Broadcaster) broadcastMessage(ctx context.Context, channel string, bytes []byte) error {
	conn := svc.redis.Get()
	defer conn.Close()

	if _, err := conn.Do("PUBLISH", channel, bytes); err != nil {
		return fmt.Errorf("publish message to channel %s, %w", channel, err)
	}
	slog.InfoContext(ctx, "broadcasted message to channel", "channel", channel)
	return nil
}

type CountEvent struct {
	Count int64 `json:"count"`
}

func (svc *Broadcaster) broadcastCountEvent(ctx context.Context, channel string, count int64) error {
	slog.InfoContext(ctx, "broadcasting count event", "channel", channel, "count", count)

	bytes, err := json.Marshal(CountEvent{Count: count})
	if err != nil {
		return fmt.Errorf("marshal count event message: %w", err)
	}
	return svc.broadcastMessage(ctx, channel, bytes)
}

func (svc *Broadcaster) BroadcastActiveCount(ctx context.Context, count int64) error {
	return svc.broadcastCountEvent(ctx, svc.names.ActiveCountChannel, count)
}

func (svc *Broadcaster) BroadcastGameCount(ctx context.Context, count int64) error {
	return svc.broadcastCountEvent(ctx, svc.names.GamesCountChannel, count)
}

func (svc *Broadcaster) BroadcastGamesEvent(ctx context.Context, output *pb.GameOutput) error {
	slog.InfoContext(ctx, "broadcasting game event", "gameOutput", output)

	bytes, err := proto.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal game event message: %w", err)
	}
	return svc.broadcastMessage(ctx, svc.names.GamesChannel, bytes)
}

func (svc *Broadcaster) BroadcastTournament(ctx context.Context, tournament *pb.TournamentOutput) error {
	slog.InfoContext(ctx, "broadcasting tournament", "tournamentOutput", tournament)

	bytes, err := proto.Marshal(tournament)
	if err != nil {
		return fmt.Errorf("marshal tournament message: %w", err)
	}
	return svc.broadcastMessage(ctx, svc.names.TournamentsChannel, bytes)
}

func (svc *Broadcaster) BroadcastChallenge(ctx context.Context, challenge model.Challenge) error {
	slog.InfoContext(ctx, "broadcasting challenge", "challenge", challenge)

	bytes, err := proto.Marshal(model.SerializeChallengeMessage(challenge))
	if err != nil {
		return fmt.Errorf("marshal user challenge message: %w", err)
	}
	return svc.broadcastMessage(ctx, svc.names.UsersChannel, bytes)
}
