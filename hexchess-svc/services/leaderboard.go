package svc

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log/slog"
	"math"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func getLeaderboardZSet(rdb *db.Redis, mode GameMode) string {
	return rdb.LeaderboardZSet + "_mode_" + string(mode)
}

type UpdtLbChangeSet struct {
	ID      int64
	EloDiff float64
}

func SetLeaderboard(ctx context.Context, rdb *db.Redis, mode GameMode, changes ...UpdtLbChangeSet) error {
	modeLbZSet := getLeaderboardZSet(rdb, mode)

	pipe := rdb.Cache.TxPipeline()
	for _, cs := range changes {
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: cs.EloDiff, Member: cs.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("'ZADD' leaderboard users: %w", err)
	}
	slog.InfoContext(ctx, "set leaderboard users", "modeLbZSet", modeLbZSet, "changes", changes)
	return nil
}

func IncrLeaderboard(ctx context.Context, rdb *db.Redis, mode GameMode, changes ...UpdtLbChangeSet) error {
	modeLbZSet := getLeaderboardZSet(rdb, mode)

	pipe := rdb.Cache.TxPipeline()
	for _, cs := range changes {
		pipe.ZIncrBy(ctx, modeLbZSet, cs.EloDiff, strconv.Itoa(int(cs.ID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("'ZINCRBY' leaderboard user: %w", err)
	}
	slog.InfoContext(ctx, "incremented leaderboard user", "modeLbZSet", modeLbZSet, "changes", changes)
	return nil
}

type Leaderboard struct {
	Users     []RankedUser `json:"users"`
	PageCount int          `json:"pageCount"`
}

type LbRank struct {
	Rank  int64   `json:"rank"`
	Score float64 `json:"score"`
}

func GetLeaderboardRanks(ctx context.Context, rdb *db.Redis, id int64, modes []GameMode) (map[GameMode]LbRank, error) {
	strID := strconv.Itoa(int(id))

	pipeline := rdb.Cache.TxPipeline()
	getExecs := make([]*redis.RankWithScoreCmd, 0, len(AllGameModes))

	for _, mode := range modes {
		getExecs = append(getExecs, rdb.Cache.ZRevRankWithScore(ctx, getLeaderboardZSet(rdb, mode), strID))
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline get ranks: %w", err)
	}

	leaderboardRanks := make(map[GameMode]LbRank)
	setRank := func(mode GameMode, rs redis.RankScore) {
		// redis ranks start from 0, hexchess ranks start from 1.
		leaderboardRanks[mode] = LbRank{Rank: rs.Rank + 1, Score: rs.Score}
	}

	type modeExec[T any] struct {
		mode GameMode
		cmd  *T
	}
	pipeline = rdb.Cache.Pipeline()
	var addExecs []modeExec[redis.IntCmd]

	for i, mode := range modes {
		isRankExists := true
		rs, err := getExecs[i].Result()
		if errors.Is(err, redis.Nil) {
			isRankExists = false
		} else if err != nil {
			return nil, fmt.Errorf("get rank for mode %v: %w", mode, err)
		}
		if isRankExists {
			setRank(mode, rs)
		} else {
			modeLbZSet := getLeaderboardZSet(rdb, mode)
			addExecs = append(addExecs, modeExec[redis.IntCmd]{
				mode: mode,
				cmd:  pipeline.ZAddNX(ctx, modeLbZSet, redis.Z{Member: strID, Score: StartElo}),
			})
			slog.InfoContext(ctx, "adding rank for mode", "mode", mode, "id", id)
		}
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline add ranks: %w", err)
	}

	pipeline = rdb.Cache.Pipeline()
	var getAfterExec []modeExec[redis.RankWithScoreCmd]

	for _, c := range addExecs {
		_, err := c.cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("add rank for mode %v: %w", c.mode, err)
		}
		getAfterExec = append(getAfterExec, modeExec[redis.RankWithScoreCmd]{
			mode: c.mode,
			cmd:  pipeline.ZRevRankWithScore(ctx, getLeaderboardZSet(rdb, c.mode), strID),
		})
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline get after add ranks: %w", err)
	}

	for _, c := range getAfterExec {
		rs, err := c.cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("get after add rank for mode %v: %w", c.mode, err)
		}
		setRank(c.mode, rs)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "id", id, "ranks", leaderboardRanks)
	return leaderboardRanks, nil
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
	lbd = Leaderboard{Users: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", count, "leaderboard", lbd)
	return lbd, nil
}

func GetLeaderboardPage(ctx context.Context, rdb *db.Redis, mode GameMode, page, perPage int64) (Leaderboard, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, mode, offset, perPage)
	util.DynLog(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}

func SyncLeaderboard(ctx context.Context, mode GameMode, dbs *db.Databases) error {
	afterID := int64(0)
	for {
		rows, err := dbs.Pdb.Query.GetEloList(ctx, db.GetEloListParams{ID: afterID, Mode: db.ModeEnum(mode), Limit: 20})
		if err != nil {
			return fmt.Errorf("select elo list: %w", err)
		}
		var changes []UpdtLbChangeSet
		for i, row := range rows {
			if i == len(rows)-1 {
				afterID = row.UserID
			}
			changes = append(changes, UpdtLbChangeSet{ID: row.UserID, EloDiff: row.Elo})
		}
		slog.InfoContext(ctx, "created update leaderboard changeset", "changes", changes, "nextAfterID", afterID)
		if len(changes) == 0 {
			break
		}
		if err := SetLeaderboard(ctx, dbs.Rdb, mode, changes...); err != nil {
			return fmt.Errorf("set leaderboard: %w", err)
		}
	}
	return nil
}
