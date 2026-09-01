package tournament

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"log/slog"
)

type TournamentNotificationService struct {
	// infra deps
	broadcaster pubsub.Broadcaster

	// service deps
	leaderboard LeaderboardGetter
}

type LeaderboardGetter interface {
	GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error)
}

func NewTournamentBroadcaster(
	leaderboard LeaderboardGetter,
	broadcaster pubsub.Broadcaster,
) *TournamentNotificationService {
	return &TournamentNotificationService{leaderboard: leaderboard, broadcaster: broadcaster}
}

func (services *TournamentNotificationService) BroadcastTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
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
