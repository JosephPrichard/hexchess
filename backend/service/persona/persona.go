package persona

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/service/replay"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type PersonaService struct {
	user        UserGetter
	leaderboard LeaderboardGetter
	replays     ReplayQuerier
}

type UserGetter interface {
	GetUserByID(ctx context.Context, id int64) (model.User, error)
	GetUserStats(ctx context.Context, id int64) (model.UserStats, error)
}

type LeaderboardGetter interface {
	GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (map[string]leaderboard.LbRank, error)
}

type ReplayQuerier interface {
	SearchReplaysByQuery(ctx context.Context, q replay.ReplaysQuery) ([]model.FullReplay, error)
}

func NewPersonaService(user UserGetter, leaderboard LeaderboardGetter, replay ReplayQuerier) *PersonaService {
	return &PersonaService{user: user, leaderboard: leaderboard, replays: replay}
}

func (services *PersonaService) GetPersona(ctx context.Context, userID int64, perPage int32) (model.Persona, error) {
	defer perf.WithContext(ctx).Log()

	var userData model.User
	var stats model.UserStats
	var replayList []model.FullReplay
	var lbRanks map[string]leaderboard.LbRank

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userData, err = services.user.GetUserByID(egCtx, userID)
		return serrors.New("get user", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		stats, err = services.user.GetUserStats(egCtx, userID)
		return serrors.New("get user stats", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		lbRanks, err = services.leaderboard.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		return serrors.New("get user leaderboard ranks", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		replayList, err = services.replays.SearchReplaysByQuery(egCtx, replay.ReplaysQuery{
			UserID:  opt.Some(userID),
			PerPage: perPage,
		})
		return serrors.New("get user replays", err, "userID", userID)
	})

	if err := eg.Wait(); err != nil {
		return model.Persona{}, err
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		rank, ok := lbRanks[modeStats.Mode.String()]
		if !ok {
			slog.WarnContext(ctx, "missing leaderboard rank for full user", "mode", modeStats.Mode)
			continue
		}
		modeStats.Rank = rank.Rank
	}

	if replayList == nil {
		replayList = []model.FullReplay{}
	}
	return model.Persona{User: userData, Stats: stats, ReplayList: replayList}, nil
}
