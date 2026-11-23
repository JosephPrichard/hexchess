package tournament

import (
	"context"
	"hexchess-svc/game/gamedata"
	"hexchess-svc/pubsub"
	"hexchess-svc/user"
	"log/slog"
)

type TournamentNotifier struct {
	// infra
	publisher pubsub.Publisher

	// service
	leaderboard TournamentLeaderboardGetter
}

type TournamentLeaderboardGetter interface {
	GetLeaderboardUser(ctx context.Context, userID int64, mode gamedata.GameMode) (user.LbdUser, error)
}

func NewTournamentNotifier(leaderboard TournamentLeaderboardGetter, publisher pubsub.Publisher) *TournamentNotifier {
	return &TournamentNotifier{leaderboard: leaderboard, publisher: publisher}
}

func (service *TournamentNotifier) NotifyTournamentParticipant(ctx context.Context, playerID int64, event JoinTournamentEvent) {
	leaderboardUser, err := service.leaderboard.GetLeaderboardUser(ctx, playerID, event.Mode)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get leaderboard user to broadcast tournament participant", "playerID", playerID, "tournamentJoin", event, "err", err)
		return
	}

	tkey := event.TournamentKey.String()
	PublishTournamentEvent(ctx, service.publisher, TournamentEvent{
		Key:             tkey,
		Kind:            TournamentParticipantKind,
		LeaderboardUser: leaderboardUser,
	})
}
