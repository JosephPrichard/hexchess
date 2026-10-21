package service

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/database/query"
	"hexchess-svc/model"
	"hexchess-svc/utils/errutil"
	"hexchess-svc/utils/perf"
	"log/slog"
	"math"
	"sort"
	"strconv"

	"hexchess-svc/utils/slogutil"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type LeaderboardService struct {
	redis   cache.Redis
	querier query.Querier
}

func NewLeaderboardService(redis cache.Redis, querier query.Querier) *LeaderboardService {
	return &LeaderboardService{redis: redis, querier: querier}
}

type SetLbChangeSet struct {
	Mode    model.GameMode
	ID      int64
	EloDiff float64
}

func (service *LeaderboardService) SetLeaderboard(ctx context.Context, changes ...SetLbChangeSet) error {
	pipe := service.redis.PrimaryClient.Pipeline()

	var outgoingChanges []SetLbChangeSet
	for _, change := range changes {
		if model.IsGuestID(change.ID) {
			continue
		}
		modeLbZSet := cache.FmtLeaderboardZSet(change.Mode.String())

		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
		outgoingChanges = append(outgoingChanges, change)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return errutil.Wrap("set leaderboard users", err)
	}

	slog.InfoContext(ctx, "set leaderboard users", "changes", outgoingChanges)

	return nil
}

type Leaderboard struct {
	RankedUsers []RankedUser `json:"users"`
	PageCount   int          `json:"pageCount"`
}

type LeaderboardRank struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
}

func MapLeaderboardRank(rank int64) int64 {
	return rank + 1
}

func (service *LeaderboardService) GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (_ map[string]LeaderboardRank, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	type getExec struct {
		mode string
		cmd  *redis.RankWithScoreCmd
	}
	type addExec struct {
		mode   model.GameMode
		addCmd *redis.IntCmd
		getCmd *redis.RankWithScoreCmd
	}

	strUserID := strconv.Itoa(int(userID))
	ranks := make(map[string]LeaderboardRank)

	// fetch and read ranks for each mode in a single pipeline
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
		ranks[exec.mode] = LeaderboardRank{Rank: MapLeaderboardRank(rankScore.Rank), Score: rankScore.Score}
	}

	// lazily initialize then fetch ranks for modes which were not retrieved in the calls above
	pipeline = service.redis.PrimaryClient.Pipeline()
	var addExecs []addExec

	for _, mode := range modes {
		if _, isRankRetrieved := ranks[mode.String()]; isRankRetrieved {
			continue
		}
		modeLbZSet := cache.FmtLeaderboardZSet(mode.String())
		addExecs = append(addExecs, addExec{
			mode:   mode,
			addCmd: pipeline.ZAddNX(ctx, modeLbZSet, redis.Z{Member: strUserID, Score: model.StartElo}),
			getCmd: pipeline.ZRevRankWithScore(ctx, modeLbZSet, strUserID),
		})
	}

	slog.InfoContext(ctx, "pipeline exec: setting ranks for user", "userID", userID)

	if err := pipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	for _, exec := range addExecs {
		if _, err := exec.addCmd.Result(); err != nil {
			return nil, err
		}
		rankScore, err := exec.getCmd.Result()
		if err != nil {
			return nil, err
		}
		ranks[exec.mode.String()] = LeaderboardRank{Rank: MapLeaderboardRank(rankScore.Rank), Score: rankScore.Score}
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "userID", userID, "ranks", ranks)
	return ranks, nil
}

func (service *LeaderboardService) getLeaderboard(ctx context.Context, mode model.GameMode, startRank, leaderboardElemCount int64) (Leaderboard, error) {
	modeLbZSet := cache.FmtLeaderboardZSet(mode.String())

	end := startRank - 1 + leaderboardElemCount
	strUserIDs, err := service.redis.PrimaryClient.ZRevRange(ctx, modeLbZSet, startRank, end).Result()
	if err != nil {
		return Leaderboard{}, errutil.Wrap("retrieve reverse leaderboard by range", err)
	}

	totalLbdElemCount, err := service.redis.PrimaryClient.ZCount(ctx, modeLbZSet, "-inf", "+inf").Result()
	if err != nil {
		return Leaderboard{}, errutil.Wrap("count leaderboard", err)
	}

	users := make([]RankedUser, 0, len(strUserIDs))
	for i, strUserID := range strUserIDs {
		userID, err := strconv.Atoi(strUserID)
		if err != nil {
			return Leaderboard{}, errutil.Wrap("parse user id", err, "strUserID", strUserID)
		}
		users = append(users, RankedUser{ID: int64(userID), Rank: startRank + int64(i) + 1})
	}

	pageCount :=
		int((totalLbdElemCount / leaderboardElemCount) +
			int64(math.Min(float64(totalLbdElemCount%leaderboardElemCount), 1)))

	leaderboard := Leaderboard{RankedUsers: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", leaderboardElemCount, "leaderboard", leaderboard)
	return leaderboard, nil
}

func (service *LeaderboardService) GetLeaderboardPage(ctx context.Context, mode model.GameMode, page, perPage int64) (_ Leaderboard, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := service.getLeaderboard(ctx, mode, offset, perPage)

	slog.Log(ctx, slogutil.Level(err), "failed to retrieved leaderboard page", "error", err, "page", page, "perPage", perPage, "leaderboard", leaderboard)
	return leaderboard, err
}

func (service *LeaderboardService) SyncLeaderboard(ctx context.Context) (err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	for _, mode := range model.GameModeEnums {
		afterID := int64(0)
		for {
			rows, err := service.querier.SelectEloList(ctx, query.SelectEloListParams{ID: afterID, Mode: query.ModeEnum(mode.String()), Limit: 20})
			if err != nil {
				return errutil.Wrap("select elo list", err, "afterID", afterID)
			}
			var changes []SetLbChangeSet
			for i, row := range rows {
				if i == len(rows)-1 {
					afterID = row.UserID
				}
				changes = append(changes, SetLbChangeSet{ID: row.UserID, Mode: mode, EloDiff: row.Elo})
			}
			slog.InfoContext(ctx, "syncing leaderboard changes", "changes", changes, "nextAfterID", afterID)
			if len(changes) == 0 {
				break
			}
			if err := service.SetLeaderboard(ctx, changes...); err != nil {
				return errutil.Wrap("set leaderboard", err)
			}
		}
	}
	return nil
}

type ExpLdbError struct {
	ExpCount    int64
	ActualCount int64
}

func (e ExpLdbError) Error() string {
	return fmt.Sprintf("expected leaderboard of length %d users, got %d", e.ExpCount, e.ActualCount)
}

func MapLeaderboardUser(row query.SelectUserWithEloByIDRow) model.LbdUser {
	return model.LbdUser{
		ID: row.ID, Username: row.Username, Country: row.Country, JoinedOn: row.JoinedOn.Time,
		Elo:        model.DefaultUserElo(row.Elo),
		HighestElo: model.DefaultUserElo(row.HighestElo),
		Wins:       row.Wins.Int32,
		Losses:     row.Losses.Int32,
		Winrate:    model.CalcUserWinrate(row.Wins.Int32, row.Losses.Int32, row.Draws.Int32),
	}
}

func (service *LeaderboardService) GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (_ model.LbdUser, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	strUserID := strconv.Itoa(int(userID))

	var userRow query.SelectUserWithEloByIDRow
	var rankScore redis.RankScore

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userRow, err = service.querier.SelectUserWithEloByID(egCtx, query.SelectUserWithEloByIDParams{
			ID:   userID,
			Mode: query.ModeEnum(mode.String()),
		})
		return errutil.Wrap("select user with elos by userID", err, "userID", userID)
	})
	eg.Go(func() (err error) {
		rankScore, err = service.redis.PrimaryClient.ZRankWithScore(egCtx, cache.FmtLeaderboardZSet(mode.String()), strUserID).Result()
		return errutil.Wrap("get user rank by userID", err, "userID", userID)
	})

	if err := eg.Wait(); err != nil {
		return model.LbdUser{}, err
	}

	leaderboardUser := MapLeaderboardUser(userRow)
	leaderboardUser.Rank = rankScore.Rank

	return leaderboardUser, nil
}

func (service *LeaderboardService) GetFullLeaderboardUsers(ctx context.Context, mode model.GameMode, rankUsers []RankedUser) (_ []model.LbdUser, _ []int64, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	if len(rankUsers) == 0 {
		return nil, nil, nil
	}

	ids := make([]int64, 0, len(rankUsers))
	for _, rankUser := range rankUsers {
		ids = append(ids, rankUser.ID)
	}
	userRows, err := service.querier.SelectUserWithEloByIDs(ctx, query.SelectUserWithEloByIDsParams{
		Ids:  ids,
		Mode: query.ModeEnum(mode.String()),
	})
	if err != nil {
		return nil, nil, errutil.Wrap("select many users", err, "userIDs", ids)
	}

	leaderboardUsers := make([]model.LbdUser, 0, len(rankUsers))
	var missingIDs []int64

	for _, rnkUser := range rankUsers {
		var foundRow *query.SelectUserWithEloByIDsRow
		for i := range userRows {
			if userRows[i].ID == rnkUser.ID {
				foundRow = &userRows[i]
				break
			}
		}
		if foundRow != nil {
			leaderboardUser := MapLeaderboardUser(query.SelectUserWithEloByIDRow(*foundRow))
			leaderboardUser.Rank = rnkUser.Rank
			leaderboardUsers = append(leaderboardUsers, leaderboardUser)
		} else {
			missingIDs = append(missingIDs, rnkUser.ID)
		}
	}

	if len(missingIDs) > 0 {
		slog.ErrorContext(ctx, "leaderboard users missing from database", "missingIDs", missingIDs)
	}
	sort.Slice(leaderboardUsers, func(i, j int) bool {
		return leaderboardUsers[i].Rank < leaderboardUsers[j].Rank
	})

	slog.InfoContext(ctx, "selected users", "ids", ids, "leaderboardUsers", leaderboardUsers, "rnkUsers", rankUsers)
	return leaderboardUsers, missingIDs, nil
}

const MaxSearchOffset = 1000

var ErrSearchLimit = errors.New("search limit exceeded")

func (service *LeaderboardService) GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) (_ []model.LbdUser, err error) {
	defer perf.WithContext(ctx).LogMaybe(err)

	if name == "" {
		return []model.LbdUser{}, nil
	}

	page = max(page, 1)
	offset := (page - 1) * perPage
	if offset > MaxSearchOffset {
		slog.WarnContext(ctx, "failed to search offset exceeds maximum", "offset", offset, "maxOffset", MaxSearchOffset)
		return []model.LbdUser{}, nil
	}

	userRows, err := service.querier.SelectUsersBySimilarity(ctx, query.SelectUsersBySimilarityParams{
		Username: name,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		return nil, errutil.Wrap("select users by similarity", err)
	}

	var userIDs []int64
	for _, row := range userRows {
		userIDs = append(userIDs, row.ID)
	}
	eloRows, err := service.querier.SelectUserElosByIDs(ctx, userIDs)
	if err != nil {
		return nil, errutil.Wrap("select elos by user ids", err, "userIDs", userIDs)
	}

	eloAggrMap := make(map[int64]struct {
		HighestElo float64
		Elo        float64
		Datapoints int
		Wins       int32
		Losses     int32
		Draws      int32
		Winrate    int64
	})
	for _, row := range eloRows {
		aggr := eloAggrMap[row.UserID]

		aggr.Elo = Average(aggr.Elo, aggr.Datapoints, row.Elo)
		aggr.HighestElo = max(aggr.HighestElo, row.Elo)
		aggr.Winrate = Average(aggr.Winrate, aggr.Datapoints, model.CalcUserWinrate(row.Wins, row.Losses, row.Draws))

		aggr.Losses += row.Losses
		aggr.Wins += row.Wins
		aggr.Draws += row.Draws

		aggr.Datapoints += 1

		eloAggrMap[row.UserID] = aggr
	}

	leaderboardUsers := make([]model.LbdUser, 0, len(userRows))
	for i, userRow := range userRows {
		searchRank := (page-1)*perPage + int32(i) + 1

		aggr := eloAggrMap[userRow.ID]

		leaderboardUsers = append(leaderboardUsers, model.LbdUser{
			User: model.User{
				ID:       userRow.ID,
				Username: userRow.Username,
				Country:  userRow.Country,
			},
			Elo:        aggr.Elo,
			HighestElo: aggr.HighestElo,
			Wins:       aggr.Wins,
			Losses:     aggr.Losses,
			Draws:      aggr.Draws,
			Winrate:    aggr.Winrate,
			Rank:       int64(searchRank),
		})
	}

	slog.InfoContext(ctx, "selected users by name similarity", "users", leaderboardUsers, "name", name, "page", page, "limit", page, "offset", offset)
	return leaderboardUsers, nil
}
