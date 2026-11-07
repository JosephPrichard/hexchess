package tournament

import (
	"context"
	"hexchess-svc/gamedata"
	"hexchess-svc/pubsub/publishers"
	"hexchess-svc/user"
	"log/slog"
)

type TournamentNotifier struct {
	// infra
	broadcaster publishers.Broadcaster

	// service
	leaderboard TournamentLeaderboardGetter
}

type TournamentLeaderboardGetter interface {
	GetLeaderboardUser(ctx context.Context, userID int64, mode gamedata.GameMode) (user.LbdUser, error)
}

func NewTournamentNotifier(leaderboard TournamentLeaderboardGetter, broadcaster publishers.Broadcaster) *TournamentNotifier {
	return &TournamentNotifier{leaderboard: leaderboard, broadcaster: broadcaster}
}

func (service *TournamentNotifier) NotifyTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
	leaderboardUser, err := service.leaderboard.GetLeaderboardUser(ctx, playerID, event.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user to broadcast tournament participant", "playerID", playerID, "tournamentJoin", event, "err", err)
		return
	}
	service.broadcaster.BroadcastTournament(ctx, TournamentOutput{
		Key:             event.TournamentKey.String(),
		Kind:            TournamentParticipantKind,
		LeaderboardUser: leaderboardUser,
	})
}
