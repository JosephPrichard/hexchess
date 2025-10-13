package dal

import (
	"context"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"hexchess-svc/app/util"
	"log/slog"
	"math"
	"strconv"
)

type IncrLbChangeSet struct {
	ID      int64
	EloDiff float64
}

func IncrLeaderboard(ctx context.Context, rdb *redis.Pool, csList ...IncrLbChangeSet) error {
	trace := ctx.Value(util.TraceKey)
	conn := rdb.Get()
	defer conn.Close()

	conn.Send("MULTI")
	for _, cs := range csList {
		conn.Send("ZINCRBY", LeaderboardZSet, cs.EloDiff, cs.ID)
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return fmt.Errorf("failed to increment leaderboard user: %w", err)
	}

	slog.Info("incremented leaderboard user", "csList", csList, "trace", trace)
	return nil
}

type Leaderboard struct {
	Users     []RankedUser `json:"users"`
	PageCount int          `json:"pageCount"`
}

func GetLeaderboardRank(ctx context.Context, rdb *redis.Pool, id int64) (int, error) {
	trace := ctx.Value(util.TraceKey)

	fail := func(str string, err error) (int, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to get leaderboard rank", "id", id, "trace", trace, "err", err)
		return 0, err
	}

	conn := rdb.Get()
	defer conn.Close()

	rank, err := redis.Int(conn.Do("ZREVRANK", LeaderboardZSet, id))
	if errors.Is(err, redis.ErrNil) {
		if _, err := conn.Do("ZINCRBY", LeaderboardZSet, StartElo, id); err != nil {
			return fail("failed to incr rank", err)
		}
		if rank, err = redis.Int(conn.Do("ZREVRANK", LeaderboardZSet, id)); err != nil {
			return fail("failed to get rank", err)
		}
	}
	if err != nil {
		return fail("failed to get rank", err)
	}

	slog.Info("retrieved leaderboard rank", "trace", trace, "id", id, "rank", rank+1)
	return rank + 1, nil
}

func GetLeaderboard(ctx context.Context, rdb *redis.Pool, startRank, count int) (Leaderboard, error) {
	trace := ctx.Value(util.TraceKey)

	fail := func(str string, err error) (Leaderboard, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to fetch leaderboard", "trace", trace, "startRank", startRank, "count", count, "err", err)
		return Leaderboard{}, err
	}

	conn := rdb.Get()
	defer conn.Close()

	end := startRank - 1 + count
	ids, err := redis.Strings(conn.Do("ZREVRANGE", LeaderboardZSet, int64(startRank), int64(end)))
	if err != nil {
		return fail("failed to get leaderboard", err)
	}
	elemCount, err := redis.Int64(conn.Do("ZCOUNT", LeaderboardZSet, "-inf", "+inf"))
	if err != nil {
		return fail("failed to count leaderboard", err)
	}

	users := make([]RankedUser, 0, len(ids))
	for i, strID := range ids {
		id, err := strconv.ParseInt(strID, 10, 64)
		if err != nil {
			return fail("failed to parse ranked ID", err)
		}
		users = append(users, RankedUser{ID: id, Rank: int64(startRank + i + 1)})
	}

	pageCount := int((elemCount / int64(count)) + int64(math.Min(float64(elemCount%int64(count)), 1)))

	leaderboard := Leaderboard{Users: users, PageCount: pageCount}
	slog.Info("retrieved leaderboard", "trace", trace, "startRank", startRank, "count", count, "leaderboard", leaderboard)
	return leaderboard, nil
}

func GetLeaderboardPage(ctx context.Context, rdb *redis.Pool, page, perPage int) (Leaderboard, error) {
	trace := ctx.Value(util.TraceKey)

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := GetLeaderboard(ctx, rdb, offset, perPage)
	util.DynLog("retrieved leaderboard page", err, "trace", trace, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}
