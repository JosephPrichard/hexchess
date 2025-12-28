package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"log/slog"
	"math"
	"sort"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func getLeaderboardZSet(rdb *db.Redis, mode GameMode) string {
	return rdb.LeaderboardZSet + "_mode_" + mode.String()
}

type UpdtLbChangeSet struct {
	Mode    GameMode
	ID      int64
	EloDiff float64
}

func SetLeaderboard(ctx context.Context, rdb *db.Redis, changes ...UpdtLbChangeSet) error {
	pipe := rdb.Cache.TxPipeline()
	for _, cs := range changes {
		modeLbZSet := getLeaderboardZSet(rdb, cs.Mode)
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: cs.EloDiff, Member: cs.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("'ZADD' leaderboard users: %w", err)
	}
	slog.InfoContext(ctx, "set leaderboard users", "changes", changes)
	return nil
}

func IncrLeaderboard(ctx context.Context, rdb *db.Redis, changes ...UpdtLbChangeSet) error {
	pipe := rdb.Cache.TxPipeline()
	for _, cs := range changes {
		pipe.ZIncrBy(ctx, getLeaderboardZSet(rdb, cs.Mode), cs.EloDiff, strconv.Itoa(int(cs.ID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("'ZINCRBY' leaderboard user: %w", err)
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

func GetLeaderboardRanks(ctx context.Context, rdb *db.Redis, id int64, modes map[string]GameMode) (map[GameMode]LbRank, error) {
	ranks := make(map[GameMode]LbRank)
	setRank := func(mode GameMode, rs redis.RankScore) {
		ranks[mode] = LbRank{Rank: rs.Rank + 1, Score: rs.Score} // redis ranks start from 0, hexchess ranks start from 1.
	}

	strID := strconv.Itoa(int(id))

	// fetch and read ranks for each mode in a single pipeline
	type getExec struct {
		mode GameMode
		cmd  *redis.RankWithScoreCmd
	}
	pipeline := rdb.Cache.Pipeline()
	getExecs := make([]getExec, 0, len(modes))

	for _, mode := range modes {
		getExecs = append(getExecs, getExec{
			mode: mode,
			cmd:  rdb.Cache.ZRevRankWithScore(ctx, getLeaderboardZSet(rdb, mode), strID),
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
	pipeline = rdb.Cache.Pipeline()
	var addExecs []addExec

	for _, mode := range modes {
		if _, ok := ranks[mode]; ok {
			continue
		}
		modeLbZSet := getLeaderboardZSet(rdb, mode)
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
		setRank(exec.mode, rs)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "id", id, "ranks", ranks)
	return ranks, nil
}

func GetLeaderboard(ctx context.Context, rdb *db.Redis, mode GameMode, startRank, count int64) (Leaderboard, error) {
	modeLbZSet := getLeaderboardZSet(rdb, mode)

	var lbd Leaderboard

	end := startRank - 1 + count
	ids, err := rdb.Cache.ZRevRange(ctx, modeLbZSet, startRank, end).Result()
	if err != nil {
		return lbd, fmt.Errorf("retrieve leaderboard by range: %w", err)
	}

	elemCount, err := rdb.Cache.ZCount(ctx, modeLbZSet, "-inf", "+inf").Result()
	if err != nil {
		return lbd, fmt.Errorf("count leaderboard: %w", err)
	}

	users := make([]RankedUser, 0, len(ids))
	for i, strID := range ids {
		id, err := strconv.ParseInt(strID, 10, 64)
		if err != nil {
			return lbd, fmt.Errorf("parse ranked ID: %w", err)
		}
		users = append(users, RankedUser{ID: id, Rank: startRank + int64(i) + 1})
	}

	pageCount := int((elemCount / count) + int64(math.Min(float64(elemCount%count), 1)))
	lbd = Leaderboard{RankedUsers: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", count, "leaderboard", lbd)
	return lbd, nil
}

func GetLeaderboardPage(ctx context.Context, rdb *db.Redis, mode GameMode, page, perPage int64) (Leaderboard, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, mode, offset, perPage)
	logutil.DynLog(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}

func SyncLeaderboard(ctx context.Context, databases *db.Databases) error {
	for _, mode := range GameModeMap {
		afterID := int64(0)
		for {
			rows, err := databases.Pdb.Query.SelectEloList(ctx, db.SelectEloListParams{ID: afterID, Mode: db.ModeEnum(mode.String()), Limit: 20})
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
			if err := SetLeaderboard(ctx, databases.Rdb, changes...); err != nil {
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

func GetLeaderboardUsers(ctx context.Context, query *db.Queries, mode GameMode, rnkUsers []RankedUser) ([]LbdUserEntity, error) {
	ids := make([]int64, 0, len(rnkUsers))
	for _, user := range rnkUsers {
		ids = append(ids, user.ID)
	}
	rows, err := query.SelectUserWithEloByIDs(ctx, db.SelectUserWithEloByIDsParams{
		Ids:  ids,
		Mode: db.ModeEnum(mode.String()),
	})
	if err != nil {
		return nil, fmt.Errorf("select many users %+v: %w", ids, err)
	}
	if len(rows) != len(rnkUsers) {
		return nil, ExpLdbError{ExpCount: int64(len(rnkUsers)), ActualCount: int64(len(rows))}
	}

	var lbdUsers []LbdUserEntity
	for _, row := range rows {
		lbdUsers = append(lbdUsers, LbdUserEntity{
			UserEntity: UserEntity{ID: row.ID, Username: row.Username, Country: row.Country, JoinedOn: row.JoinedOn.Time},
			Elo:        row.Elo,
			HighestElo: row.HighestElo,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Winrate:    calcUserWinrate(row.Wins, row.Losses),
		})
	}

	for i := range lbdUsers {
		lbdUser := &lbdUsers[i]
		for _, rnkUser := range rnkUsers {
			if rnkUser.ID == lbdUser.ID {
				lbdUser.Rank = rnkUser.Rank
				break
			}
		}
	}
	sort.Slice(lbdUsers, func(i, j int) bool {
		return lbdUsers[i].Rank < lbdUsers[j].Rank
	})

	slog.InfoContext(ctx, "selected users", "ids", ids, "ldbUsers", lbdUsers, "rnkUsers", rnkUsers)
	return lbdUsers, nil
}
