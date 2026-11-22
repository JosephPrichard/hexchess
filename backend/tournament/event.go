package tournament

import (
	"context"
	"encoding/json/v2"
	"hexchess-svc/pubsub"
	"hexchess-svc/pubsub/event"
	"hexchess-svc/user"
	"log/slog"
)

type TournamentEventKind string

const (
	TournamentParticipantKind TournamentEventKind = "participant"
	TournamentCountdownKind   TournamentEventKind = "countdown"
	TournamentStartKind       TournamentEventKind = "start"
	TournamentMatchmakingKind TournamentEventKind = "matchmaking"
	TournamentErrorKind       TournamentEventKind = "error"
)

type TournamentEvent struct {
	Key             string              `json:"tournamentKey"`
	Kind            TournamentEventKind `json:"kind"`
	Matches         []FullMatch         `json:"matches"`
	Error           string              `json:"error"`
	LeaderboardUser user.LbdUser        `json:"leaderboardUser"`
}

func PublishTournamentEvent(ctx context.Context, publisher pubsub.Publisher, e TournamentEvent) {
	slog.InfoContext(ctx, "PubSub: publishing tournament event", "event", e)

	bytes, err := json.Marshal(e)
	if err != nil {
		slog.ErrorContext(ctx, "PubSub: failed to marshal tournament event", "error", err)
		return
	}
	publisher.Publish(ctx, event.TournamentsChannel, e.Key, bytes)
}
