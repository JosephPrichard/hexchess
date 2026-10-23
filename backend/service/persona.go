package service

import (
	"context"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/utils/opt"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/slogutil"
	"log/slog"

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
	defer perf.WithContext(ctx).LogFailable(err)

	var userData model.User
	var userDataErr error

	var stats model.UserStats
	var statsErr error

	var replayList []model.FullReplay
	var replayListErr error

	leaderboardRanks := make(map[string]LeaderboardRank)
	var leaderboardRanksErr error

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		userData, userDataErr = service.user.GetUserByID(egCtx, userID)
		if userDataErr != nil {
			userDataErr = serrors.New("get user", userDataErr, "userID", userID)
		}
		return nil
	})

	eg.Go(func() error {
		stats, statsErr = service.user.GetUserStats(egCtx, userID)
		if statsErr != nil {
			statsErr = serrors.New("get user stats", statsErr, "userID", userID)
		}
		return nil
	})

	eg.Go(func() error {
		leaderboardRanks, replayListErr = service.leaderboard.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		if replayListErr != nil {
			replayListErr = serrors.New("get user leaderboard ranks", replayListErr, "userID", userID)
		}
		return nil
	})

	eg.Go(func() error {
		replayList, leaderboardRanksErr = service.replays.SearchReplaysByQuery(egCtx, ReplaysQuery{
			UserID:  opt.Some(userID),
			PerPage: perPage,
		})
		if leaderboardRanksErr != nil {
			leaderboardRanksErr = serrors.New("get user replays", leaderboardRanksErr, "userID", userID)
		}
		return nil
	})

	// wait but not check the error, we do not propagate through error context so all operations will finish even if any fails
	eg.Wait()

	// each error is checked in a deterministic order to produce a deterministic result in case of multiple calls failing
	var errors = []error{userDataErr, statsErr, replayListErr, leaderboardRanksErr}

	slog.Log(ctx, slogutil.ErrorsLevel(errors), "completed get persona operation",
		"errors", errors, "userData", userData, "stats", stats, "leaderboardRanks", leaderboardRanks, "replayList", replayList)

	for _, err := range errors {
		if err != nil {
			return model.Persona{}, err
		}
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		rank, ok := leaderboardRanks[modeStats.Mode.String()]
		if !ok {
			// assert: leaderboard ranks must be valid for each mode
			return model.Persona{}, fmt.Errorf("user %d leaderboard rank is missing for mode: %s", userID, modeStats.Mode)
		}
		modeStats.Rank = rank.Rank
	}

	if replayList == nil {
		replayList = []model.FullReplay{}
	}
	return model.Persona{User: userData, Stats: stats, ReplayList: replayList}, nil
}
