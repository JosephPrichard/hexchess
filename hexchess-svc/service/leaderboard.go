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
	"hexchess-svc/util/logutil"

	"github.com/redis/go-redis/v9"
)

type UpdtLbChangeSet struct {
	Mode    GameMode
	ID      int64
	EloDiff float64
}

func (svc *HexchessServices) SetLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := svc.redis.Cache.TxPipeline()
	for _, change := range changes {
		if IsGuestID(change.ID) {
			continue
		}
		modeLbZSet := svc.getLeaderboardZSet(change.Mode.String())
		pipe.ZAddNX(ctx, modeLbZSet, redis.Z{Score: change.EloDiff, Member: change.ID})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set leaderboard users: %w", err)
	}
	slog.InfoContext(ctx, "set leaderboard users", "changes", changes)
	return nil
}

func (svc *HexchessServices) incrLeaderboard(ctx context.Context, changes ...UpdtLbChangeSet) error {
	pipe := svc.redis.Cache.TxPipeline()
	for _, change := range changes {
		if IsGuestID(change.ID) || change.EloDiff == 0 {
			// noop zero value changes.
			continue
		}
		modeLbZSet := svc.getLeaderboardZSet(change.Mode.String())
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

func mapLbRank(rank int64) int64 {
	return rank + 1
}

func (svc *HexchessServices) GetUserLeaderboardRanks(ctx context.Context, userID int64, modes map[string]GameMode) (map[string]LbRank, error) {
	ranks := make(map[string]LbRank)
	setRank := func(mode string, rankScore redis.RankScore) {
		ranks[mode] = LbRank{Rank: mapLbRank(rankScore.Rank), Score: rankScore.Score} // rdb ranks start from 0, hexchess ranks start from 1.
	}

	strID := strconv.Itoa(int(userID))

	// fetch and read ranks for each mode in a single pipeline
	type getExec struct {
		mode string
		cmd  *redis.RankWithScoreCmd
	}
	pipeline := svc.redis.Cache.Pipeline()
	getExecs := make([]getExec, 0, len(modes))

	for _, mode := range modes {
		modeLbZSet := svc.getLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			mode: mode.String(),
			cmd:  svc.redis.Cache.ZRevRankWithScore(ctx, modeLbZSet, strID),
		})
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline get leaderboard ranks: %w", err)
	}
	for _, exec := range getExecs {
		rankScore, err := exec.cmd.Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get leaderboard rank for mode %v: %w", exec.mode, err)
		}
		setRank(exec.mode, rankScore)
	}

	// lazily initialize then fetch ranks for modes which were not included in the calls above
	type addExec struct {
		mode   GameMode
		addCmd *redis.IntCmd
		getCmd *redis.RankWithScoreCmd
	}
	pipeline = svc.redis.Cache.Pipeline()
	var addExecs []addExec

	for _, mode := range modes {
		if _, ok := ranks[mode.String()]; ok {
			continue
		}
		modeLbZSet := svc.getLeaderboardZSet(mode.String())
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
		rankScore, err := exec.getCmd.Result()
		if err != nil {
			return nil, fmt.Errorf("get after add leaderboard rank for mode %v: %w", exec.mode, err)
		}
		setRank(exec.mode.String(), rankScore)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "id", userID, "ranks", ranks)
	return ranks, nil
}

func (svc *HexchessServices) getUsersLeaderboardRank(ctx context.Context, userIDs []int64, mode GameMode) (map[int64]int64, error) {
	type getExec struct {
		userID int64
		cmd    *redis.IntCmd
	}

	pipeline := svc.redis.Cache.Pipeline()
	var getExecs []getExec

	for _, userID := range userIDs {
		modeLbZSet := svc.getLeaderboardZSet(mode.String())
		getExecs = append(getExecs, getExec{
			userID: userID,
			cmd:    pipeline.ZRevRank(ctx, modeLbZSet, strconv.Itoa(int(userID))),
		})
	}

	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, fmt.Errorf("exec pipeline get leaderboard ranks: %w", err)
	}

	leaderboardRanks := make(map[int64]int64)
	for _, exec := range getExecs {
		rank, err := exec.cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("get leaderboard rank for user %v: %w", exec.userID, err)
		}
		leaderboardRanks[exec.userID] = mapLbRank(rank)
	}

	slog.InfoContext(ctx, "retrieved leaderboard ranks", "leaderboardRanks", leaderboardRanks, "mode", mode)
	return leaderboardRanks, nil
}

func (svc *HexchessServices) getLeaderboard(ctx context.Context, mode GameMode, startRank, count int64) (Leaderboard, error) {
	modeLbZSet := svc.getLeaderboardZSet(mode.String())

	end := startRank - 1 + count
	ids, err := svc.redis.Cache.ZRevRange(ctx, modeLbZSet, startRank, end).Result()
	if err != nil {
		return Leaderboard{}, fmt.Errorf("retrieve reverse leaderboard by range: %w", err)
	}

	elemCount, err := svc.redis.Cache.ZCount(ctx, modeLbZSet, "-inf", "+inf").Result()
	if err != nil {
		return Leaderboard{}, fmt.Errorf("count leaderboard: %w", err)
	}

	users := make([]RankedUser, 0, len(ids))
	for i, strID := range ids {
		id, err := strconv.ParseInt(strID, 10, 64)
		if err != nil {
			return Leaderboard{}, fmt.Errorf("parse ranked Key: %w", err)
		}
		users = append(users, RankedUser{ID: id, Rank: startRank + int64(i) + 1})
	}

	pageCount := int((elemCount / count) + int64(math.Min(float64(elemCount%count), 1)))
	leaderboard := Leaderboard{RankedUsers: users, PageCount: pageCount}

	slog.InfoContext(ctx, "retrieved leaderboard", "modeLbZSet", modeLbZSet, "startRank", startRank, "count", count, "leaderboard", leaderboard)
	return leaderboard, nil
}

func (svc *HexchessServices) GetLeaderboardPage(ctx context.Context, mode GameMode, page, perPage int64) (Leaderboard, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	leaderboard, err := svc.getLeaderboard(ctx, mode, offset, perPage)
	logutil.DynLog(ctx, "retrieved leaderboard page", err, "page", page, "perPage", perPage, "leaderboard", leaderboard, "err", err)
	return leaderboard, err
}

func (svc *HexchessServices) SyncLeaderboard(ctx context.Context) error {
	for _, mode := range GameModeEnums {
		afterID := int64(0)
		for {
			rows, err := svc.querier.SelectEloList(ctx, sqlc.SelectEloListParams{ID: afterID, Mode: sqlc.ModeEnum(mode.String()), Limit: 20})
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

type LbdUserDTO struct {
	UserDTO
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

func mapLbdUser(row sqlc.SelectUserWithEloByIDsRow) LbdUserDTO {
	return LbdUserDTO{
		UserDTO:    UserDTO{ID: row.ID, Username: row.Username, Country: row.Country, JoinedOn: row.JoinedOn.Time},
		Elo:        defaultElo(row.Elo),
		HighestElo: defaultElo(row.HighestElo),
		Wins:       row.Wins.Int32,
		Losses:     row.Losses.Int32,
		Winrate:    calcUserWinrate(row.Wins.Int32, row.Losses.Int32, row.Draws.Int32),
	}
}

func (svc *HexchessServices) GetLeaderboardUsers(ctx context.Context, mode GameMode, rnkUsers []RankedUser) ([]LbdUserDTO, []int64, error) {
	ids := make([]int64, 0, len(rnkUsers))
	for _, user := range rnkUsers {
		ids = append(ids, user.ID)
	}
	userRows, err := svc.querier.SelectUserWithEloByIDs(ctx, sqlc.SelectUserWithEloByIDsParams{
		Ids:  ids,
		Mode: sqlc.ModeEnum(mode.String()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("select many users %+v: %w", ids, err)
	}

	leaderboardUsers := make([]LbdUserDTO, 0, len(rnkUsers))
	var missingIDs []int64

	for _, rnkUser := range rnkUsers {
		var foundRow *sqlc.SelectUserWithEloByIDsRow
		for i := range userRows {
			if userRows[i].ID == rnkUser.ID {
				foundRow = &userRows[i]
				break
			}
		}
		if foundRow != nil {
			user := mapLbdUser(*foundRow)
			user.Rank = rnkUser.Rank
			leaderboardUsers = append(leaderboardUsers, user)
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

func (svc *HexchessServices) GetFuzzySearchLeaderboard(ctx context.Context, name string, page, perPage int32) ([]LbdUserDTO, error) {
	page = max(page, 1)
	offset := (page - 1) * perPage
	if offset > MaxSearchOffset {
		slog.WarnContext(ctx, "failed to search offset exceeds maximum", "offset", offset, "maxOffset", MaxSearchOffset)
		return nil, ErrSearchLimit
	}

	userRows, err := svc.querier.SelectUsersBySimilarity(ctx, sqlc.SelectUsersBySimilarityParams{
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
	eloRows, err := svc.querier.SelectUserElosByIDs(ctx, userIDs)
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

	leaderboardUsers := make([]LbdUserDTO, 0, len(userRows))
	for i, userRow := range userRows {
		searchRank := (page-1)*perPage + int32(i) + 1

		aggr := eloAggrMap[userRow.ID]

		leaderboardUsers = append(leaderboardUsers, LbdUserDTO{
			UserDTO: UserDTO{
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
