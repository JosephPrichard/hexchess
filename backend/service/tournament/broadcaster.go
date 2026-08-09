package tournament

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"log/slog"
)

type TournamentBroadcaster struct {
	leaderboard LeaderboardGetter
	broadcaster pubsub.Broadcaster
}

type LeaderboardGetter interface {
	GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error)
}

func NewTournamentBroadcaster(
	leaderboard LeaderboardGetter,
	broadcaster pubsub.Broadcaster,
) *TournamentBroadcaster {
	return &TournamentBroadcaster{leaderboard: leaderboard, broadcaster: broadcaster}
}

func (services *TournamentBroadcaster) BroadcastTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
	leaderboardUser, err := services.leaderboard.GetLeaderboardUser(ctx, playerID, event.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user to broadcast tournament participant", "playerID", playerID, "tournamentJoin", event, "err", err)
		return
	}
	services.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:             event.TournamentKey.String(),
		Kind:            model.TournamentParticipantKind,
		LeaderboardUser: leaderboardUser,
	})
}
