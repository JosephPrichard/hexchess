package persona

import (
	"context"
	"errors"
	"hexchess-svc/cache"
	"hexchess-svc/database"
	"hexchess-svc/database/query"
	"hexchess-svc/gamedata"
	"hexchess-svc/replay"
	"hexchess-svc/user"
	"hexchess-svc/userdata"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type Persona struct {
	User       user.User           `json:"user"`
	Stats      user.UserStats      `json:"stats"`
	ReplayList []replay.FullReplay `json:"replayList"`
}

type PersonaService struct {
	// infra
	database.PostgreSQL
	redis cache.Redis
}

func NewPersonaService(database database.PostgreSQL, redis cache.Redis) *PersonaService {
	return &PersonaService{PostgreSQL: database, redis: redis}
}

func (service *PersonaService) GetPersona(ctx context.Context, userID int64, perPage int32) (_ Persona, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	var userData user.User
	var stats user.UserStats
	var replayList []replay.FullReplay
	leaderboardRanks := make(map[string]PersonaLeaderboardRank)

	var userDataErr error

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		// special case: get user by id operation will not be cancelled by error group cancellation
		userData, err = service.getPersonaUserByID(ctx, userID)
		if err != nil {
			userDataErr = serrors.New("get user", err, "userID", userID)
		}
		return userDataErr
	})

	eg.Go(func() (err error) {
		stats, err = service.getPersonaUserStats(egCtx, userID)
		return serrors.New("get user stats", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		leaderboardRanks, err = service.getPersonaLeaderboardRanks(egCtx, userID, gamedata.GameModeEnums)
		return serrors.New("get user leaderboard ranks", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		replayList, err = service.getReplaysByQuery(egCtx, userID, perPage)
		return serrors.New("get user replays", err, "userID", userID)
	})

	egErr := eg.Wait()
	if userDataErr != nil {
		// special case: return user error first if it exists, contains "prioritized" error codes such as NotFound, etc.
		return Persona{}, userDataErr
	}
	if egErr != nil {
		return Persona{}, egErr
	}

	slog.InfoContext(ctx, "completed get persona operation", "userData", userData, "stats", stats, "leaderboardRanks", leaderboardRanks, "replayList", replayList)

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
		replayList = []replay.FullReplay{}
	}
	return Persona{User: userData, Stats: stats, ReplayList: replayList}, nil
}

func (service *PersonaService) getPersonaUserByID(ctx context.Context, id int64) (_ user.User, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	userRow, err := service.Querier.SelectUserByID(ctx, id)
	if database.IsErrNoRows(err) {
		return user.User{}, user.ErrUserNotFound
	} else if err != nil {
		return user.User{}, serrors.New("select user", err, "userID", id)
	}

	u := user.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}

	slog.InfoContext(ctx, "selected persona by id", "userID", id, "user", u)
	return u, nil
}

func (service *PersonaService) getPersonaUserStats(ctx context.Context, userID int64) (_ user.UserStats, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	modeEloRows, err := service.Querier.SelectUserElosByID(ctx, userID)
	if err != nil {
		return user.UserStats{}, serrors.New("select user elos by id", err, "userID", userID)
	}

	var stats user.UserStats
	if len(modeEloRows) == 0 {
		stats = user.UserStats{HighestElo: userdata.StartElo, AvgElo: userdata.StartElo}
	} else {
		stats = user.UserStats{HighestElo: math.SmallestNonzeroFloat64}
	}

	for _, row := range modeEloRows {
		mode := enum.Expect(row.Mode, gamedata.GameModeEnums)
		modeStats := user.ModeStats{
			Mode:       mode,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			Winrate:    userdata.CalcUserWinrate(row.Wins, row.Losses, row.Draws),
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

func (service *PersonaService) getPersonaLeaderboardRanks(ctx context.Context, userID int64, modes map[string]gamedata.GameMode) (_ map[string]PersonaLeaderboardRank, err error) {
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

func (service *PersonaService) getReplaysByQuery(ctx context.Context, userID int64, perPage int32) (_ []replay.FullReplay, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	params := query.SelectReplaysByQueryParams{
		PerPage: perPage,
		UserID:  pgtype.Int8{Int64: userID, Valid: true},
		// search cursor is unused
		AfterID: math.MaxInt64,
		// default sort key means "sort by id" for the 'ORDER BY' in the SQL query
		SortKey: "",
	}
	replayRows, err := service.Querier.SelectReplaysByQuery(ctx, params)
	if err != nil {
		return nil, serrors.New("select replays by userID and perPage", err, "userID", userID, "perPage", perPage)
	}

	replays := make([]replay.FullReplay, 0, len(replayRows))
	for _, row := range replayRows {
		replays = append(replays, mapFullReplayByIDRow(query.SelectReplayByIDRow(row)))
	}

	slog.InfoContext(ctx, "selected replays", "userID", userID, "perPage", perPage, "replays", replays)
	return replays, nil
}

func mapFullReplayByIDRow(row query.SelectReplayByIDRow) replay.FullReplay {
	replayResult := enum.Expect(row.Result, replay.ReplayResultEnums)
	replayCause := enum.Expect(row.Cause, replay.ReplayCauseEnums)
	gameMode := enum.Expect(row.Mode, gamedata.GameModeEnums)

	replayData := replay.Replay{
		ID:          row.ID,
		WhiteID:     row.WhiteID.Int64,
		BlackID:     row.BlackID.Int64,
		Result:      replayResult,
		Cause:       replayCause,
		Mode:        gameMode,
		WinEloDiff:  row.WinEloDiff,
		LoseEloDiff: row.LoseEloDiff,
		PlayedOn:    row.PlayedOn.Time,
		Rating:      row.Rating.Float64,
		TurnCount:   int(row.TurnCount),
	}

	return replay.FullReplay{
		Replay: replayData,
		ReplayUsers: replay.ReplayUsers{
			WhiteName:    row.WhiteName.String,
			BlackName:    row.BlackName.String,
			WhiteCountry: row.WhiteCountry.String,
			BlackCountry: row.BlackCountry.String,
			WhiteElo:     userdata.DefaultUserElo(row.WhiteElo),
			BlackElo:     userdata.DefaultUserElo(row.BlackElo),
		},
		ReplayColorElos: replay.NewReplayView(replayData),
	}
}
