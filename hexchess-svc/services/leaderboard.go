package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"

	"hexchess-svc/db/sqlc"
	"hexchess-svc/pkg/logutil"

	"github.com/redis/go-redis/v9"
)

type UpdtLbChangeSet struct {
	Mode    GameMode
	ID      int64
	EloDiff float64
}

func (svc *Services) SetLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := svc.Redis.Cache.TxPipeline()
	for _, change := range changes {
		modeLbZSet := svc.GetLeaderboardZSet(change.Mode.String())
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set leaderboard users: %w", err)
	}
	slog.InfoContext(ctx, "set leaderboard users", "changes", changes)
	return nil
}

func (svc *Services) IncrLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := svc.Redis.Cache.TxPipeline()
	for _, change := range changes {
		modeLbZSet := svc.GetLeaderboardZSet(change.Mode.String())
		pipe.ZIncrBy(ctx, modeLbZSet, change.EloDiff, strconv.Itoa(int(change.ID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("incr leaderboard user: %w", err)
	}
	slog.InfoContext(ctx, "incremented leaderboard user", "changes", changes)
	return nil
}

type Leaderboard struct {
	RankedUsers []RankedUser `json:"users"`
	PageCount   int          `json:"pageCount"`
}

type LbRank struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
}

func (svc *Services) GetLeaderboardRanks(ctx context.Context, id int64, modes map[string]GameMode) (map[string]LbRank, error) {
	ranks := make(map[string]LbRank)
	setRank := func(mode string, rs redis.RankScore) {
		ranks[mode] = LbRank{Rank: rs.Rank + 1, Score: rs.Score} // rdb ranks start from 0, hexchess ranks start from 1.
	}

	strID := strconv.Itoa(int(id))

	// fetch and read ranks for each mode in a single pipeline
	type getExec struct {
		mode string
		cmd  *redis.RankWithScoreCmd
	}
	pipeline := svc.Redis.Cache.Pipeline()
	getExecs := make([]getExec, 0, len(modes))

	for _, mode := range modes {
		modeLbZSet := svc.GetLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			mode: mode.String(),
			cmd:  svc.Redis.Cache.ZRevRankWithScore(ctx, modeLbZSet, strID),
		})
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline get leaderboard ranks: %w", err)
	}
	for _, exec := range getExecs {
		rs, err := exec.cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get leaderboard rank for mode %v: %w", exec.mode, err)
		}
		setRank(exec.mode, rs)
	}

	// lazily initialize then fetch ranks for modes which were not included in the calls above
	type addExec struct {
		mode   GameMode
		addCmd *redis.IntCmd
		getCmd *redis.RankWithScoreCmd
	}
	pipeline = svc.Redis.Cache.Pipeline()
	var addExecs []addExec

	for _, mode := range modes {
		if _, ok := ranks[mode.String()]; ok {
			continue
		}
		modeLbZSet := svc.GetLeaderboardZSet(mode.String())
		addExecs = append(addExecs, addExec{
			mode:   mode,
			addCmd: pipeline.ZAddNX(ctx, modeLbZSet, redis.Z{Member: strID, Score: StartElo}),
			getCmd: pipeline.ZRevRankWithScore(ctx, modeLbZSet, strID),
		})
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline add leaderboard ranks: %w", err)
	}
	for _, exec := range addExecs {
		if _, err := exec.addCmd.Result(); err != nil {
			return nil, fmt.Errorf("add leaderboard rank for mode %v: %w", exec.mode, err)
		}
		rs, err := exec.getCmd.Result()
		if err != nil {
			return nil, fmt.Errorf("get after add leaderboard rank for mode %v: %w", exec.mode, err)
		}
		setRank(exec.mode.String(), rs)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "id", id, "ranks", ranks)
	return ranks, nil
}

func (svc *Services) GetLeaderboard(ctx context.Context, mode GameMode, startRank, count int64) (Leaderboard, error) {
	modeLbZSet := svc.GetLeaderboardZSet(mode.String())

	var leaderboard Leaderboard

	end := startRank - 1 + count
	ids, err := svc.Redis.Cache.ZRevRange(ctx, modeLbZSet, startRank, end).Result()
	if err != nil {
		return leaderboard, fmt.Errorf("retrieve reverse leaderboard by range: %w", err)
	}

	elemCount, err := svc.Redis.Cache.ZCount(ctx, modeLbZSet, "-inf", "+inf").Result()
	if err != nil {
		return leaderboard, fmt.Errorf("count leaderboard: %w", err)
	}

	users := make([]RankedUser, 0, len(ids))
	for i, strID := range ids {
		id, err := strconv.ParseInt(strID, 10, 64)
		if err != nil {
			return leaderboard, fmt.Errorf("parse ranked ID: %w", err)
		}
		users = append(users, RankedUser{ID: id, Rank: startRank + int64(i) + 1})
	}

	pageCount := int((elemCount / count) + int64(math.Min(float64(elemCount%count), 1)))
	leaderboard = Leaderboard{RankedUsers: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", count, "leaderboard", leaderboard)
	return leaderboard, nil
}

func (svc *Services) GetLeaderboardPage(ctx context.Context, mode GameMode, page, perPage int64) (Leaderboard, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := svc.GetLeaderboard(ctx, mode, offset, perPage)
	logutil.DynLog(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}

func (svc *Services) SyncLeaderboard(ctx context.Context) error {
	for _, mode := range GameModeMap {
		afterID := int64(0)
		for {
			rows, err := svc.Queries.SelectEloList(ctx, sqlc.SelectEloListParams{ID: afterID, Mode: sqlc.ModeEnum(mode.String()), Limit: 20})
			if err != nil {
				return fmt.Errorf("select elo list afterID %d: %w", afterID, err)
			}
			var changes []UpdtLbChangeSet
			for i, row := range rows {
				if i == len(rows)-1 {
					afterID = row.UserID
				}
				changes = append(changes, UpdtLbChangeSet{Mode: mode, ID: row.UserID, EloDiff: row.Elo})
			}
			slog.InfoContext(ctx, "created update leaderboard changeset", "changes", changes, "nextAfterID", afterID)
			if len(changes) == 0 {
				break
			}
			if err := svc.SetLeaderboard(ctx, changes...); err != nil {
				return fmt.Errorf("set leaderboard: %w", err)
			}
		}
	}
	return nil
}

type LbdUserEntity struct {
	UserEntity
	Elo        float64 `json:"elo"`
	HighestElo float64 `json:"highestElo"`
	Wins       int32   `json:"wins"`
	Losses     int32   `json:"losses"`
	Draws      int32   `json:"draws"`
	Winrate    int64   `json:"winrate"`
	Rank       int64   `json:"rank"`
}

type ExpLdbError struct {
	ExpCount    int64
	ActualCount int64
}

func (e ExpLdbError) Error() string {
	return fmt.Sprintf("expected leaderboard of length %d users, got %d", e.ExpCount, e.ActualCount)
}

func (svc *Services) GetLeaderboardUsers(ctx context.Context, mode GameMode, rnkUsers []RankedUser) ([]LbdUserEntity, []int64, error) {
	ids := make([]int64, 0, len(rnkUsers))
	for _, user := range rnkUsers {
		ids = append(ids, user.ID)
	}
	userRows, err := svc.Queries.SelectUserWithEloByIDs(ctx, sqlc.SelectUserWithEloByIDsParams{
		Ids:  ids,
		Mode: sqlc.ModeEnum(mode.String()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("select many users %+v: %w", ids, err)
	}

	var leaderboardUsers []LbdUserEntity
	var missingIDs []int64

	for _, rnkUser := range rnkUsers {
		var found *sqlc.SelectUserWithEloByIDsRow
		for i := range userRows {
			if userRows[i].ID == rnkUser.ID {
				found = &userRows[i]
				break
			}
		}
		if found != nil {
			leaderboardUsers = append(leaderboardUsers, LbdUserEntity{
				UserEntity: UserEntity{ID: rnkUser.ID, Username: found.Username, Country: found.Country, JoinedOn: found.JoinedOn.Time},
				Elo:        defaultElo(found.Elo),
				HighestElo: defaultElo(found.HighestElo),
				Wins:       found.Wins.Int32,
				Losses:     found.Losses.Int32,
				Winrate:    calcUserWinrate(found.Wins.Int32, found.Losses.Int32, found.Draws.Int32),
				Rank:       rnkUser.Rank,
			})
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

	slog.InfoContext(ctx, "selected users", "ids", ids, "leaderboardUsers", leaderboardUsers, "rnkUsers", rnkUsers)
	return leaderboardUsers, missingIDs, nil
}

const MaxSearchOffset = 1000

var ErrSearchLimit = errors.New("search limit exceeded")

func (svc *Services) GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) ([]LbdUserEntity, error) {
	page = max(page, 1)
	offset := (page - 1) * perPage
	if offset > MaxSearchOffset {
		slog.WarnContext(ctx, "failed to search offset exceeds maximum", "offset", offset, "maxOffset", MaxSearchOffset)
		return nil, ErrSearchLimit
	}

	userRows, err := svc.Queries.SelectUsersBySimilarity(ctx, sqlc.SelectUsersBySimilarityParams{
		Username: name,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("select users by similarity: %w", err)
	}

	var userIDs []int64
	for _, row := range userRows {
		userIDs = append(userIDs, row.ID)
	}
	eloRows, err := svc.Queries.SelectManyUserElosById(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("select elos by user ids %v: %w", userIDs, err)
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

		aggr.Elo = avg(aggr.Elo, aggr.Datapoints, row.Elo)
		aggr.HighestElo = max(aggr.HighestElo, row.Elo)
		aggr.Winrate = avg(aggr.Winrate, aggr.Datapoints, calcUserWinrate(row.Wins, row.Losses, row.Draws))

		aggr.Losses += row.Losses
		aggr.Wins += row.Wins
		aggr.Draws += row.Draws

		aggr.Datapoints += 1

		eloAggrMap[row.UserID] = aggr
	}

	var users []LbdUserEntity
	for i, userRow := range userRows {
		searchRank := (page-1)*perPage + int32(i) + 1

		aggr := eloAggrMap[userRow.ID]

		users = append(users, LbdUserEntity{
			UserEntity: UserEntity{
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

	slog.InfoContext(ctx, "selected users by name similarity", "users", users, "name", name, "page", page, "limit", page, "offset", offset)
	return users, nil
}
