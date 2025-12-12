package dpl

import (
	"context"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"log/slog"
	"math"
	"strconv"

	"github.com/gomodule/redigo/redis"
)

type UpdtLbChangeSet struct {
	ID      int64
	EloDiff float64
}

func SetLeaderboard(ctx context.Context, rdb *infra.Redis, changes ...UpdtLbChangeSet) error {
	conn := rdb.Cache.Get()
	defer conn.Close()

	conn.Send("MULTI")
	for _, cs := range changes {
		conn.Send("ZADD", rdb.LeaderboardZSet, "NX", cs.EloDiff, cs.ID)
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return fmt.Errorf("'ZADD' leaderboard users: %w", err)
	}

	slog.InfoContext(ctx, "set leaderboard users", "changes", changes)
	return nil
}

func IncrLeaderboard(ctx context.Context, rdb *infra.Redis, changes ...UpdtLbChangeSet) error {
	conn := rdb.Cache.Get()
	defer conn.Close()

	conn.Send("MULTI")
	for _, cs := range changes {
		conn.Send("ZINCRBY", rdb.LeaderboardZSet, cs.EloDiff, cs.ID)
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return fmt.Errorf("'ZINCRBY' leaderboard user: %w", err)
	}

	slog.InfoContext(ctx, "incremented leaderboard user", "changes", changes)
	return nil
}

type Leaderboard struct {
	Users     []RankedUser `json:"users"`
	PageCount int          `json:"pageCount"`
}

func GetLeaderboardRank(ctx context.Context, rdb *infra.Redis, id int64) (int64, error) {
	conn := rdb.Cache.Get()
	defer conn.Close()

	rank, err := redis.Int64(conn.Do("ZREVRANK", rdb.LeaderboardZSet, id))
	if errors.Is(err, redis.ErrNil) {
		rank = -1
	} else if err != nil {
		return 0, fmt.Errorf("get leaderboard rank: %w", err)
	}
	if rank == -1 {
		if _, err := conn.Do("ZINCRBY", rdb.LeaderboardZSet, StartElo, id); err != nil {
			return 0, fmt.Errorf("incr leaderboard rank: %w", err)
		}
		if rank, err = redis.Int64(conn.Do("ZREVRANK", rdb.LeaderboardZSet, id)); err != nil {
			return 0, fmt.Errorf("get leaderboard rank: %w", err)
		}
	}

	slog.InfoContext(ctx, "retrieved leaderboard rank", "id", id, "rank", rank+1)
	return rank + 1, nil
}

func GetLeaderboard(ctx context.Context, rdb *infra.Redis, startRank, count int64) (Leaderboard, error) {
	var lbd Leaderboard

	conn := rdb.Cache.Get()
	defer conn.Close()

	end := startRank - 1 + count
	ids, err := redis.Strings(conn.Do("ZREVRANGE", rdb.LeaderboardZSet, startRank, end))
	if err != nil {
		return lbd, fmt.Errorf("retrieve leaderboard by range: %w", err)
	}
	elemCount, err := redis.Int64(conn.Do("ZCOUNT", rdb.LeaderboardZSet, "-inf", "+inf"))
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
	slog.InfoContext(ctx, "retrieved leaderboard", "startRank", startRank, "count", count, "leaderboard", lbd)
	return lbd, nil
}

func GetLeaderboardPage(ctx context.Context, rdb *infra.Redis, page, perPage int64) (Leaderboard, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, offset, perPage)
	util.DynLog(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}

func SyncLeaderboard(ctx context.Context, dbs *infra.Databases) error {
	afterID := int64(0)
	for {
		rows, err := dbs.Pdb.Query.SelectEloListAfterID(ctx, db.SelectEloListAfterIDParams{AfterID: afterID, Limit: 20})
		if err != nil {
			return fmt.Errorf("select elo list: %w", err)
		}
		var changes []UpdtLbChangeSet
		for i, row := range rows {
			if i == len(rows)-1 {
				afterID = row.ID
			}
			changes = append(changes, UpdtLbChangeSet{ID: row.ID, EloDiff: row.Elo})
		}
		slog.InfoContext(ctx, "created update leaderboard changeset", "changes", changes, "nextAfterID", afterID)
		if len(changes) == 0 {
			break
		}
		if err := SetLeaderboard(ctx, dbs.Rdb, changes...); err != nil {
			return fmt.Errorf("incr leaderboard: %w", err)
		}
	}
	return nil
}
