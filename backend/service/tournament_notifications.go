package svc

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"log/slog"
)

type TournamentNotificationsService struct {
	leaderboardService *LeaderboardService
	broadcaster        pubsub.Broadcaster
}

func NewTournamentParticipantService(leaderboardService *LeaderboardService, broadcaster pubsub.Broadcaster) *TournamentNotificationsService {
	return &TournamentNotificationsService{leaderboardService: leaderboardService, broadcaster: broadcaster}
}

func (services *TournamentNotificationsService) SendTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
	leaderboardUser, err := services.leaderboardService.GetLeaderboardUser(ctx, playerID, event.Mode)
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
