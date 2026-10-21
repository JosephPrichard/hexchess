package service

import (
	"context"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"

	"golang.org/x/sync/errgroup"
)

type PersonaService struct {
	user        PersonaUserGetter
	leaderboard LeaderboardGetter
	replays     ReplayQuerier
}

type PersonaUserGetter interface {
	GetUserByID(ctx context.Context, id int64) (model.User, error)
	GetUserStats(ctx context.Context, id int64) (model.UserStats, error)
}

type LeaderboardGetter interface {
	GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (map[string]LeaderboardRank, error)
}

type ReplayQuerier interface {
	SearchReplaysByQuery(ctx context.Context, q ReplaysQuery) ([]model.FullReplay, error)
}

func NewPersonaService(user PersonaUserGetter, leaderboard LeaderboardGetter, replay ReplayQuerier) *PersonaService {
	return &PersonaService{user: user, leaderboard: leaderboard, replays: replay}
}

func (service *PersonaService) GetPersona(ctx context.Context, userID int64, perPage int32) (_ model.Persona, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	var userData model.User
	var stats model.UserStats
	var replayList []model.FullReplay
	leaderboardRanks := make(map[string]LeaderboardRank)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userData, err = service.user.GetUserByID(egCtx, userID)
		return errutil.Wrap(
			"get user", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		stats, err = service.user.GetUserStats(egCtx, userID)
		return errutil.Wrap(
			"get user stats", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		leaderboardRanks, err = service.leaderboard.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		return errutil.Wrap(
			"get user leaderboard ranks", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		replayList, err = service.replays.SearchReplaysByQuery(egCtx, ReplaysQuery{
			UserID:  opt.Some(userID),
			PerPage: perPage,
		})
		return errutil.Wrap("get user replays", err, "userID", userID)
	})

	if err := eg.Wait(); err != nil {
		return model.Persona{}, err
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		rank, ok := leaderboardRanks[modeStats.Mode.String()]
		if !ok {
			return model.Persona{}, fmt.Errorf("user %d leaderboard rank is missing for mode: %s", userID, modeStats.Mode)
		}
		modeStats.Rank = rank.Rank
	}

	if replayList == nil {
		replayList = []model.FullReplay{}
	}
	return model.Persona{User: userData, Stats: stats, ReplayList: replayList}, nil
}
