package service

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/pubsub"
	"log/slog"
)

type TournamentNotifier struct {
	// infra
	broadcaster pubsub.Broadcaster

	// service
	leaderboard TournamentLeaderboardGetter
}

type TournamentLeaderboardGetter interface {
	GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error)
}

func NewTournamentNotifier(
	leaderboard TournamentLeaderboardGetter,
	broadcaster pubsub.Broadcaster,
) *TournamentNotifier {
	return &TournamentNotifier{leaderboard: leaderboard, broadcaster: broadcaster}
}

func (service *TournamentNotifier) NotifyTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
	leaderboardUser, err := service.leaderboard.GetLeaderboardUser(ctx, playerID, event.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user to broadcast tournament participant", "playerID", playerID, "tournamentJoin", event, "err", err)
		return
	}
	service.broadcaster.BroadcastTournament(ctx, model.TournamentOutput{
		Key:             event.TournamentKey.String(),
		Kind:            model.TournamentParticipantKind,
		LeaderboardUser: leaderboardUser,
	})
}
