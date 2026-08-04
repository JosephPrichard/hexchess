package svc

import (
	"context"
	"hexchess-svc/model"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type FullUserService struct {
	userService        *UserService
	leaderboardService *LeaderboardService
	replayService      *ReplayService
}

func NewFullUserService(userService *UserService, leaderboardService *LeaderboardService, replayService *ReplayService) *FullUserService {
	return &FullUserService{userService: userService, leaderboardService: leaderboardService, replayService: replayService}
}

type FullUser struct {
	User       model.User         `json:"user"`
	Stats      model.UserStats    `json:"stats"`
	ReplayList []model.FullReplay `json:"replayList"`
}

func (services *FullUserService) GetFullUser(ctx context.Context, userID int64, perPage int32) (FullUser, error) {
	defer perf.WithContext(ctx).Log()

	var user model.User
	var stats model.UserStats
	var replayList []model.FullReplay
	var lbRanks map[string]LbRank

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		user, err = services.userService.GetUserByID(egCtx, userID)
		return serrors.New("get user", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		stats, err = services.userService.GetUserStats(egCtx, userID)
		return serrors.New("get user stats", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		lbRanks, err = services.leaderboardService.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		return serrors.New("get user leaderboard ranks", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		replayList, err = services.replayService.SearchReplaysByQuery(egCtx, ReplaysQuery{
			UserID:  optional.Just(userID),
			PerPage: perPage,
		})
		return serrors.New("get user replays", err, "userID", userID)
	})

	if err := eg.Wait(); err != nil {
		return FullUser{}, err
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		lbRank, ok := lbRanks[modeStats.Mode.String()]
		if !ok {
			slog.WarnContext(ctx, "missing leaderboard rank for full user", "mode", modeStats.Mode)
			continue
		}
		modeStats.Rank = lbRank.Rank
	}

	if replayList == nil {
		replayList = []model.FullReplay{}
	}
	return FullUser{User: user, Stats: stats, ReplayList: replayList}, nil
}
