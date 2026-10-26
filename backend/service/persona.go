package service

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"math"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type PersonaService struct {
	// infra
	database.Database
	redis cache.Redis
}

func NewPersonaService(database database.Database, redis cache.Redis) *PersonaService {
	return &PersonaService{Database: database, redis: redis}
}

func (service *PersonaService) GetPersona(ctx context.Context, userID int64, perPage int32) (_ model.Persona, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	var userData model.User
	var userDataErr error

	var stats model.UserStats
	var statsErr error

	var replayList []model.FullReplay
	var replayListErr error

	leaderboardRanks := make(map[string]PersonaLeaderboardRank)
	var leaderboardRanksErr error

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userData, err = service.getPersonaUserByID(egCtx, userID)
		if err != nil {
			userDataErr = serrors.New("get user", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		stats, err = service.getPersonaUserStats(egCtx, userID)
		if err != nil {
			statsErr = serrors.New("get user stats", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		leaderboardRanks, err = service.getPersonaLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		if err != nil {
			replayListErr = serrors.New("get user leaderboard ranks", err, "userID", userID)
		}
		return nil
	})

	eg.Go(func() (err error) {
		replayList, err = service.getReplaysByQuery(egCtx, userID, perPage)
		if err != nil {
			leaderboardRanksErr = serrors.New("get user replays", err, "userID", userID)
		}
		return nil
	})

	// wait but not check the error, we do not propagate through error context so all operations will finish even if any fails
	eg.Wait()

	var errorList = []error{userDataErr, statsErr, replayListErr, leaderboardRanksErr}

	slog.Log(ctx, slogutil.ErrorsLevel(errorList), "completed get persona operation",
		"errors", errorList, "userData", userData, "stats", stats, "leaderboardRanks", leaderboardRanks, "replayList", replayList)

	for _, err := range errorList {
		if err != nil {
			return model.Persona{}, err
		}
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		mode := modeStats.Mode.String()
		rank, rankExists := leaderboardRanks[mode]
		if rankExists {
			modeStats.Rank = rank.Rank
			modeStats.Score = rank.Score
		} else {
			// leaderboard ranks must be valid for each mode (do not fail this endpoint, just assign the zero values and log for observability)
			slog.WarnContext(ctx, "leaderboard rank is missing for mode", "userID", userID, "rank", rank, "mode", mode)
		}
	}

	if replayList == nil {
		replayList = []model.FullReplay{}
	}
	return model.Persona{User: userData, Stats: stats, ReplayList: replayList}, nil
}

func (service *PersonaService) getPersonaUserByID(ctx context.Context, id int64) (_ model.User, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	userRow, err := service.Querier.SelectUserByID(ctx, id)
	if database.IsErrNoRows(err) {
		return model.User{}, ErrUserNotFound
	} else if err != nil {
		return model.User{}, serrors.New("select user", err, "userID", id)
	}

	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}

	slog.InfoContext(ctx, "selected persona by id", "userID", id, "user", user)
	return user, nil
}

func (service *PersonaService) getPersonaUserStats(ctx context.Context, userID int64) (_ model.UserStats, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	modeEloRows, err := service.Querier.SelectUserElosByID(ctx, userID)
	if err != nil {
		return model.UserStats{}, serrors.New("select user elos by id", err, "userID", userID)
	}

	var stats model.UserStats
	if len(modeEloRows) == 0 {
		stats = model.UserStats{HighestElo: model.StartElo, AvgElo: model.StartElo}
	} else {
		stats = model.UserStats{HighestElo: math.SmallestNonzeroFloat64}
	}

	for _, row := range modeEloRows {
		mode := enum.Expect(row.Mode, model.GameModeEnums)
		modeStats := model.ModeStats{
			Mode:       mode,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			Winrate:    model.CalcUserWinrate(row.Wins, row.Losses, row.Draws),
			Elo:        row.Elo,
			HighestElo: row.HighestElo,
		}

		stats.TotalWins += modeStats.Wins
		stats.TotalLosses += modeStats.Losses
		stats.TotalDraws += modeStats.Draws

		stats.HighestElo = max(stats.HighestElo, modeStats.HighestElo)
		stats.AvgElo = average(stats.AvgElo, len(stats.ModeStats), modeStats.Elo)
		stats.TotalWinrate = average(stats.TotalWinrate, len(stats.ModeStats), modeStats.Winrate)

		stats.ModeStats = append(stats.ModeStats, modeStats)
	}

	slog.InfoContext(ctx, "selected persona user elos", "stats", stats)
	return stats, nil
}

type PersonaLeaderboardRank struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
}

func mapLeaderboardRank(rank int64) int64 {
	return rank + 1
}

func (service *PersonaService) getPersonaLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (_ map[string]PersonaLeaderboardRank, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	type getExec struct {
		mode string
		cmd  *redis.RankWithScoreCmd
	}

	strUserID := strconv.Itoa(int(userID))

	ranks := make(map[string]PersonaLeaderboardRank)

	pipeline := service.redis.PrimaryClient.Pipeline()
	getExecs := make([]getExec, 0, len(modes))

	for _, mode := range modes {
		modeLbZSet := cache.FmtLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			mode: mode.String(),
			cmd:  pipeline.ZRevRankWithScore(ctx, modeLbZSet, strUserID),
		})
	}

	slog.InfoContext(ctx, "pipeline exec: retrieving ranks for user", "userID", userID)

	if err := pipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	for _, exec := range getExecs {
		rankScore, err := exec.cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		} else if err != nil {
			return nil, err
		}
		ranks[exec.mode] = PersonaLeaderboardRank{Rank: mapLeaderboardRank(rankScore.Rank), Score: rankScore.Score}
	}

	slog.InfoContext(ctx, "retrieved persona leaderboard ranks", "userID", userID, "ranks", ranks)
	return ranks, nil
}

func (service *PersonaService) getReplaysByQuery(ctx context.Context, userID int64, perPage int32) (_ []model.FullReplay, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	params := query.SelectReplaysByQueryParams{
		PerPage: perPage,
		UserID:  pgtype.Int8{Int64: userID, Valid: true},
		// search cursor is unused
		AfterID: math.MaxInt64,
		// static sort key determines the 'ORDER BY' in the SQL query (sort by id)
		SortKey: "id",
	}
	replayRows, err := service.Querier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, serrors.New("select replays by userID and perPage", err, "userID", userID, "perPage", perPage)
	}

	replays := make([]model.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(query.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "userID", userID, "perPage", perPage, "replays", replays)
	return replays, nil
}
