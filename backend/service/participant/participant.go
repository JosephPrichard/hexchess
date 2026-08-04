package participant

import (
	"context"
	"hexchess-svc/model"
	svc "hexchess-svc/service/leaderboard"
	"hexchess-svc/service/replay"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/optional"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"

	"golang.org/x/sync/errgroup"
)

type ParticipantService struct {
	userService        *user.UserService
	leaderboardService *svc.LeaderboardService
	replayService      *replay.ReplayService
}

func NewParticipantService(userService *user.UserService, leaderboardService *svc.LeaderboardService, replayService *replay.ReplayService) *ParticipantService {
	return &ParticipantService{userService: userService, leaderboardService: leaderboardService, replayService: replayService}
}

type FullUser struct {
	User       model.User         `json:"user"`
	Stats      model.UserStats    `json:"stats"`
	ReplayList []model.FullReplay `json:"replayList"`
}

func (services *ParticipantService) GetFullUser(ctx context.Context, userID int64, perPage int32) (FullUser, error) {
	defer perf.WithContext(ctx).Log()

	var userData model.User
	var stats model.UserStats
	var replayList []model.FullReplay
	var lbRanks map[string]svc.LbRank

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userData, err = services.userService.GetUserByID(egCtx, userID)
		if err != nil {
			return serrors.New("get user", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		stats, err = services.userService.GetUserStats(egCtx, userID)
		if err != nil {
			return serrors.New("get user stats", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		lbRanks, err = services.leaderboardService.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		if err != nil {
			return serrors.New("get user leaderboard ranks", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		replayList, err = services.replayService.SearchReplaysByQuery(egCtx, replay.ReplaysQuery{
			UserID:  optional.Some(userID),
			PerPage: perPage,
		})
		if err != nil {
			return serrors.New("get user replays", err, "userID", userID)
		}
		return nil
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
	return FullUser{User: userData, Stats: stats, ReplayList: replayList}, nil
}
