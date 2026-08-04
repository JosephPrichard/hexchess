package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/db/query"
	"hexchess-svc/model"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/perf"
	"log/slog"
	"math"
	"sort"
	"strconv"

	"hexchess-svc/utils/logutil"
	"hexchess-svc/utils/serrors"

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

type UpdtLbChangeSet struct {
	Mode    model.GameMode
	ID      int64
	EloDiff float64
}

type SetLbChangeSet struct {
	ID      int64
	EloDiff float64
}

func (services *LeaderboardService) SetLeaderboard(ctx context.Context, mode model.GameMode, changes ...SetLbChangeSet) error {
	pipe := services.redis.PrimaryClient.Pipeline()

	modeLbZSet := services.redis.FmtLeaderboardZSet(mode.String())

	var outgoingChanges []SetLbChangeSet
	for _, change := range changes {
		if model.IsGuestID(change.ID) {
			continue
		}
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
		outgoingChanges = append(outgoingChanges, change)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.New("set leaderboard users", err)
	}

	slog.InfoContext(ctx, "set leaderboard users", "modeLbZSet", modeLbZSet, "changes", outgoingChanges)

	return nil
}

func (services *LeaderboardService) IncrLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := services.redis.PrimaryClient.Pipeline()
	for _, change := range changes {
		if model.IsGuestID(change.ID) || change.EloDiff == 0 {
			// noop zero value changes
			continue
		}
		modeLbZSet := services.redis.FmtLeaderboardZSet(change.Mode.String())
		pipe.ZIncrBy(ctx, modeLbZSet, change.EloDiff, strconv.Itoa(int(change.ID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return serrors.New("incr leaderboard user", err)
	}
	slog.InfoContext(ctx, "incremented leaderboard user", "changes", changes)
	return nil
}

type Leaderboard struct {
	RankedUsers []user.RankedUser `json:"users"`
	PageCount   int               `json:"pageCount"`
}

type LbRank struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
}

func MapLeaderboardRank(rank int64) int64 {
	return rank + 1
}

func (services *LeaderboardService) GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]model.GameMode) (map[string]LbRank, error) {
	defer perf.WithContext(ctx).Log()

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
	ranks := make(map[string]LbRank)

	// step 1: fetch and read ranks for each mode in a single pipeline
	pipeline := services.redis.PrimaryClient.Pipeline()
	getExecs := make([]getExec, 0, len(modes))

	for _, mode := range modes {
		modeLbZSet := services.redis.FmtLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			mode: mode.String(),
			cmd:  pipeline.ZRevRankWithScore(ctx, modeLbZSet, strUserID),
		})
	}

	slog.InfoContext(ctx, "pipeline exec: retrieving ranks for user", "userID", userID)

	if err := cache.PipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	for _, exec := range getExecs {
		rankScore, err := exec.cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		} else if err != nil {
			return nil, serrors.New("get leaderboard rank", err)
		}
		ranks[exec.mode] = LbRank{Rank: MapLeaderboardRank(rankScore.Rank), Score: rankScore.Score}
	}

	// step 2: lazily initialize then fetch ranks for modes which were not retrieved in the calls above
	pipeline = services.redis.PrimaryClient.Pipeline()
	var addExecs []addExec

	for _, mode := range modes {
		if _, isRankRetrieved := ranks[mode.String()]; isRankRetrieved {
			continue
		}
		modeLbZSet := services.redis.FmtLeaderboardZSet(mode.String())
		addExecs = append(addExecs, addExec{
			mode:   mode,
			addCmd: pipeline.ZAddNX(ctx, modeLbZSet, redis.Z{Member: strUserID, Score: model.StartElo}),
			getCmd: pipeline.ZRevRankWithScore(ctx, modeLbZSet, strUserID),
		})
	}

	slog.InfoContext(ctx, "pipeline exec: setting ranks for user", "userID", userID)

	if err := cache.PipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	for _, exec := range addExecs {
		if _, err := exec.addCmd.Result(); err != nil {
			return nil, serrors.New("add leaderboard rank", err)
		}
		rankScore, err := exec.getCmd.Result()
		if err != nil {
			return nil, serrors.New("get leaderboard rank", err)
		}
		ranks[exec.mode.String()] = LbRank{Rank: MapLeaderboardRank(rankScore.Rank), Score: rankScore.Score}
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "userID", userID, "ranks", ranks)
	return ranks, nil
}

func (services *LeaderboardService) GetUsersLeaderboardRank(ctx context.Context, userIDs []int64, mode model.GameMode) (map[int64]int64, error) {
	type getExec struct {
		userID int64
		cmd    *redis.IntCmd
	}

	pipeline := services.redis.PrimaryClient.Pipeline()

	var getExecs []getExec
	for _, userID := range userIDs {
		modeLbZSet := services.redis.FmtLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			userID: userID,
			cmd:    pipeline.ZRevRank(ctx, modeLbZSet, strconv.Itoa(int(userID))),
		})
	}

	if err := cache.PipelineExec(ctx, pipeline); err != nil {
		return nil, err
	}

	leaderboardRanks := make(map[int64]int64)
	for _, exec := range getExecs {
		rank, err := exec.cmd.Result()
		if errors.Is(redis.Nil, err) {
			// skip populating this rank if we cannot retrieve it (stays at zero value)
			continue
		}
		if err != nil {
			return nil, serrors.New("get leaderboard rank for user", err, "userID", exec.userID)
		}
		leaderboardRanks[exec.userID] = MapLeaderboardRank(rank)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "leaderboardRanks", leaderboardRanks, "mode", mode)
	return leaderboardRanks, nil
}

func (services *LeaderboardService) getLeaderboard(ctx context.Context, mode model.GameMode, startRank, leaderboardElemCount int64) (Leaderboard, error) {
	modeLbZSet := services.redis.FmtLeaderboardZSet(mode.String())

	end := startRank - 1 + leaderboardElemCount
	strUserIDs, err := services.redis.PrimaryClient.ZRevRange(ctx, modeLbZSet, startRank, end).Result()
	if err != nil {
		return Leaderboard{}, serrors.New("retrieve reverse leaderboard by range", err)
	}

	totalLbdElemCount, err := services.redis.PrimaryClient.ZCount(ctx, modeLbZSet, "-inf", "+inf").Result()
	if err != nil {
		return Leaderboard{}, serrors.New("count leaderboard", err)
	}

	users := make([]user.RankedUser, 0, len(strUserIDs))
	for i, strUserID := range strUserIDs {
		userID, err := strconv.Atoi(strUserID)
		if err != nil {
			return Leaderboard{}, serrors.New("parse user id", err, "strUserID", strUserID)
		}
		users = append(users, user.RankedUser{ID: int64(userID), Rank: startRank + int64(i) + 1})
	}

	pageCount :=
		int((totalLbdElemCount / leaderboardElemCount) +
			int64(math.Min(float64(totalLbdElemCount%leaderboardElemCount), 1)))

	leaderboard := Leaderboard{RankedUsers: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", leaderboardElemCount, "leaderboard", leaderboard)
	return leaderboard, nil
}

func (services *LeaderboardService) GetLeaderboardPage(ctx context.Context, mode model.GameMode, page, perPage int64) (Leaderboard, error) {
	defer perf.WithContext(ctx).Log()

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := services.getLeaderboard(ctx, mode, offset, perPage)
	logutil.Log(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard)
	return leaderboard, err
}

func (services *LeaderboardService) SyncLeaderboard(ctx context.Context) error {
	defer perf.WithContext(ctx).Log()

	for _, mode := range model.GameModeEnums {
		afterID := int64(0)
		for {
			rows, err := services.querier.SelectEloList(ctx, query.SelectEloListParams{ID: afterID, Mode: query.ModeEnum(mode.String()), Limit: 20})
			if err != nil {
				return serrors.New("select elo list", err, "afterID", afterID)
			}
			var changes []SetLbChangeSet
			for i, row := range rows {
				if i == len(rows)-1 {
					afterID = row.UserID
				}
				changes = append(changes, SetLbChangeSet{ID: row.UserID, EloDiff: row.Elo})
			}
			slog.InfoContext(ctx, "syncing leaderboard changes", "changes", changes, "nextAfterID", afterID)
			if len(changes) == 0 {
				break
			}
			if err := services.SetLeaderboard(ctx, mode, changes...); err != nil {
				return serrors.New("set leaderboard", err)
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
		User:       model.User{ID: row.ID, Username: row.Username, Country: row.Country, JoinedOn: row.JoinedOn.Time},
		Elo:        model.DefaultUserElo(row.Elo),
		HighestElo: model.DefaultUserElo(row.HighestElo),
		Wins:       row.Wins.Int32,
		Losses:     row.Losses.Int32,
		Winrate:    model.CalcUserWinrate(row.Wins.Int32, row.Losses.Int32, row.Draws.Int32),
	}
}

func (services *LeaderboardService) GetLeaderboardUser(ctx context.Context, userID int64, mode model.GameMode) (model.LbdUser, error) {
	defer perf.WithContext(ctx).Log()

	strUserID := strconv.Itoa(int(userID))

	var userRow query.SelectUserWithEloByIDRow
	var rankScore redis.RankScore

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		userRow, err = services.querier.SelectUserWithEloByID(egCtx, query.SelectUserWithEloByIDParams{
			ID:   userID,
			Mode: query.ModeEnum(mode.String()),
		})
		return serrors.New("select user with elos by userID", err, "userID", userID)
	})
	eg.Go(func() (err error) {
		rankScore, err = services.redis.PrimaryClient.ZRankWithScore(egCtx, services.redis.FmtLeaderboardZSet(mode.String()), strUserID).Result()
		return serrors.New("get user rank by userID", err, "userID", userID)
	})

	if err := eg.Wait(); err != nil {
		return model.LbdUser{}, err
	}

	leaderboardUser := MapLeaderboardUser(userRow)
	leaderboardUser.Rank = rankScore.Rank

	return leaderboardUser, nil
}

func (services *LeaderboardService) GetFullLeaderboardUsers(ctx context.Context, mode model.GameMode, rankUsers []user.RankedUser) ([]model.LbdUser, []int64, error) {
	defer perf.WithContext(ctx).Log()

	if len(rankUsers) == 0 {
		return nil, nil, nil
	}

	ids := make([]int64, 0, len(rankUsers))
	for _, rankUser := range rankUsers {
		ids = append(ids, rankUser.ID)
	}
	userRows, err := services.querier.SelectUserWithEloByIDs(ctx, query.SelectUserWithEloByIDsParams{
		Ids:  ids,
		Mode: query.ModeEnum(mode.String()),
	})
	if err != nil {
		return nil, nil, serrors.New("select many users", err, "userIDs", ids)
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

func (services *LeaderboardService) GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) ([]model.LbdUser, error) {
	defer perf.WithContext(ctx).Log()

	if name == "" {
		return []model.LbdUser{}, nil
	}

	page = max(page, 1)
	offset := (page - 1) * perPage
	if offset > MaxSearchOffset {
		slog.WarnContext(ctx, "failed to search offset exceeds maximum", "offset", offset, "maxOffset", MaxSearchOffset)
		return []model.LbdUser{}, nil
	}

	userRows, err := services.querier.SelectUsersBySimilarity(ctx, query.SelectUsersBySimilarityParams{
		Username: name,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		return nil, serrors.New("select users by similarity", err)
	}

	var userIDs []int64
	for _, row := range userRows {
		userIDs = append(userIDs, row.ID)
	}
	eloRows, err := services.querier.SelectUserElosByIDs(ctx, userIDs)
	if err != nil {
		return nil, serrors.New("select elos by user ids", err, "userIDs", userIDs)
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

		aggr.Elo = user.Average(aggr.Elo, aggr.Datapoints, row.Elo)
		aggr.HighestElo = max(aggr.HighestElo, row.Elo)
		aggr.Winrate = user.Average(aggr.Winrate, aggr.Datapoints, model.CalcUserWinrate(row.Wins, row.Losses, row.Draws))

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
