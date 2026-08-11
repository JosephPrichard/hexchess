package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"hexchess-svc/cache"
	"hexchess-svc/chess"
	"hexchess-svc/database"
	"hexchess-svc/database/mutator"
	"hexchess-svc/pubsub"
	"hexchess-svc/service/challenge"
	"hexchess-svc/service/gameplay"
	chessSvc "hexchess-svc/service/gamestate"
	"hexchess-svc/service/leaderboard"
	"hexchess-svc/service/replay"
	"hexchess-svc/service/tournament"
	"hexchess-svc/service/user"
	"hexchess-svc/utils/entropy"
	"hexchess-svc/utils/perf"

	"hexchess-svc/model"
	"hexchess-svc/utils/alog"
	"hexchess-svc/utils/config"
	"log"
	"log/slog"
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

var (
	usersCount       = flag.Int("usersCount", 1000, "number of users to seed")
	challengesCount  = flag.Int("challengesCount", 100, "number of challenges to seed")
	gameResultCount  = flag.Int("gameResultCount", 5000, "number of game results to seed")
	tournamentsCount = flag.Int("tournamentsCount", 1000, "number of tournaments to seed")

	deterministicUsernames = flag.Bool("deterministicUsernames", true, "whether usernames follow the pattern 'User0', 'User1', etc. or are random")

	initialTimeGamesRaw   = flag.String("initialTimeGames", "2025-01-01", "the oldest date at which generated game results start from")
	gameDurationOffsetRaw = flag.String("gameDurationOffset", "24h", "the offset between the time each consecutive game is played on (e.g. 1h, 5m)")

	initialTimeGames   = time.Time{}
	gameDurationOffset = time.Duration(0)
)

const (
	ServiceName = "hexchess-seed"
	Concurrency = 64
)

func main() {
	start := time.Now()
	ctx := context.WithValue(context.Background(), alog.Trace, "seed-databases-script")

	// step 1: parse input flags and config from script input
	cfg := config.Load()

	// parse: input data parameters to generate seeded data backend
	t, err := time.Parse(time.DateOnly, *initialTimeGamesRaw)
	if err != nil {
		alog.Fatal("failed to parse date", err, "initialTimeGameResults", *initialTimeGamesRaw)
	}
	initialTimeGames = t

	d, err := time.ParseDuration(*gameDurationOffsetRaw)
	if err != nil {
		alog.Fatal("failed to parse duration", err, "gameDurationOffset", *gameDurationOffsetRaw)
	}
	gameDurationOffset = d

	// validation: check that it is reasonable to generate this number of challenges
	maxChallengePermutations := *usersCount * (*usersCount - 1)
	maxChallengesCount := maxChallengePermutations / 10
	if *challengesCount > maxChallengesCount {
		log.Fatalf("challenges count is too large, must be at most %d", maxChallengesCount)
	}

	// step 2: connect to backend infrastructure and prepare cleanup
	shutdown := alog.InitLoggers(ServiceName, cfg.OltpEndpoint, cfg.Profile)
	defer shutdown()

	databaseClient := database.NewDatabase(ctx, database.DatabaseConfig{
		ReadWriteDsn:  cfg.DbURL,
		ActiveProfile: cfg.Profile,
		AwsRegion:     cfg.AwsRegion,
	})
	defer databaseClient.Close()

	redisClient := cache.NewRedis(ctx, cache.RedisConfig{
		PrimaryAddr:   cfg.RedisPrimaryNodes,
		ActiveProfile: cfg.Profile,
	})
	defer redisClient.Close()

	if err := redisClient.PrimaryClient.FlushAll(ctx).Err(); err != nil {
		alog.Fatal("flush rdb", err)
	}

	// step 3: execute the test seed script and measure results
	services := newServices(redisClient, databaseClient)

	// root node in the foreign key hierarchy tree
	seedUsers(ctx, services.user)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return seedChallenges(egCtx, services.challenge)
	})
	eg.Go(func() error {
		return seedGameResults(egCtx, services.gameover, generateGameResults())
	})
	eg.Go(func() error {
		return seedTournaments(egCtx, databaseClient.QuerierMutator(), generateTournaments())
	})

	if err := eg.Wait(); err != nil {
		alog.Fatal("failed to seed user dependent rows", err)
	}

	// syncs the stat updates written in the game results into the leaderboard.
	if err := services.leaderboard.SyncLeaderboard(ctx); err != nil {
		alog.Fatal("jobs leaderboard", err)
	}

	slog.Info("finished seeding databases", "timeTaken", time.Since(start).String())
}

type Services struct {
	leaderboard *leaderboard.LeaderboardService
	user        *user.UserService
	challenge   *challenge.ChallengeService
	replay      *replay.ReplayService
	gameover    *gameplay.GameOverService
}

func newServices(
	redisClient cache.Redis,
	databaseClient database.Database,
) Services {
	leaderboardSvc := leaderboard.NewLeaderboardService(redisClient, databaseClient.Querier())
	userSvc := user.NewUserService(databaseClient)
	challengeSvc := challenge.NewChallengeService(databaseClient, entropy.RealSource{})
	gameoverSvc := gameplay.NewGameoverService(
		databaseClient,
		redisClient,
		// producer and broadcaster won't be invoked in the specific codepath needed to seed the data
		nil,
		pubsub.Broadcaster{},
	)
	return Services{
		leaderboard: leaderboardSvc,
		user:        userSvc,
		challenge:   challengeSvc,
		gameover:    gameoverSvc,
	}
}

func seedUsers(ctx context.Context, userSvc *user.UserService) {
	defer perf.New().Log()

	if _, err := userSvc.BatchInsertUsers(ctx, generateUserInsts()); err != nil {
		alog.Fatal("insert users", err)
	}
}

func generateUserInsts() []user.Inst {
	var insts []user.Inst
	for i := range *usersCount {
		var username string
		if *deterministicUsernames {
			username = fmt.Sprintf("User%d", i)
		} else {
			username = gofakeit.Username()
		}

		insts = append(insts, user.Inst{
			Username: username,
			Password: "password1",
			Country:  "us",
			JoinedOn: time.Now(),
		})
	}
	return insts
}

func generateUserID(useListedIDs map[int64]struct{}) int64 {
	for range 10 {
		userID := rand.Int63n(int64(*usersCount)) + 1
		if useListedIDs == nil {
			return userID
		}
		if _, used := useListedIDs[userID]; !used {
			return userID
		}
	}
	alog.Fatal("generate user id (all are uselisted)", nil)
	return 0
}

func generateUserIDPairs() (int64, int64) {
	firstID := generateUserID(nil)
	blackID := generateUserID(nil)
	if firstID == blackID {
		blackID = (blackID+1)%int64(*usersCount) + 1
	}
	return firstID, blackID
}

func generateMode() model.GameMode {
	switch rand.Intn(3) {
	case 0:
		return model.ModeCorrespondence1
	case 1:
		return model.ModeCorrespondence7
	default:
		return model.ModeCorrespondence14
	}
}

func seedChallenges(ctx context.Context, services *challenge.ChallengeService) error {
	defer perf.New().Log()
	return services.BatchInsertChallenges(ctx, generateChallengeInsts())
}

func generateChallengeInsts() []challenge.Inst {
	hashChallengeKey := func(challengerID, challengeeID int64) string {
		return fmt.Sprintf("%d,%d", challengerID, challengeeID)
	}

	var insts []challenge.Inst
	for range *challengesCount {
		// generate two challenges that are unique, this is done by retrying if a duplicate is found.
		// note(Joseph): we assume the number of users is large enough to avoid duplicates

		var challengeSet = map[string]struct{}{}
		var challengerID, challengeeID int64
		for {
			challengerID, challengeeID = generateUserIDPairs()
			key := hashChallengeKey(challengerID, challengeeID)
			if _, keyExists := challengeSet[key]; !keyExists {
				challengeSet[key] = struct{}{}
				break
			}
		}

		insts = append(insts, challenge.Inst{
			ChallengerID: challengerID,
			ChallengeeID: challengeeID,
			StartColor:   model.Random,
			Mode:         generateMode(),
			MadeOn:       time.Now(),
		})
	}
	return insts
}

type GameResultInsts struct {
	WhiteID      int64              `json:"whiteId"`
	BlackID      int64              `json:"blackId"`
	ReplayCause  model.ReplayCause  `json:"cause"`
	ReplayResult model.ReplayResult `json:"result"`
	ReplayMode   model.GameMode     `json:"mode"`
}

func generateReplayResult() model.ReplayResult {
	switch rand.Intn(3) {
	case 0:
		return model.WhiteWin
	case 1:
		return model.Draw
	default:
		return model.BlackWin
	}
}

func generateGameResults() []GameResultInsts {
	var insts []GameResultInsts
	for range *gameResultCount {
		whiteID, blackID := generateUserIDPairs()
		insts = append(insts, GameResultInsts{
			WhiteID:      whiteID,
			BlackID:      blackID,
			ReplayCause:  model.Checkmate,
			ReplayResult: generateReplayResult(),
			ReplayMode:   generateMode(),
		})
	}
	return insts
}

func seedGameResults(ctx context.Context, gameoverSvc *gameplay.GameOverService, insts []GameResultInsts) error {
	defer perf.New().Log()

	a := insts
	rand.Shuffle(len(a), func(i, j int) {
		a[i], a[j] = a[j], a[i]
	})

	eg, egCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, Concurrency)

	for gameIdx, inst := range insts {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			mode := inst.ReplayMode

			moveSeq, err := chessSvc.RandomMoveHistSeq(mode, 10, 30, -1)
			if err != nil {
				return fmt.Errorf("generate random move seq: %w", err)
			}

			_, err = gameoverSvc.InsertFinishedGame(egCtx, model.FinishGameEvent{
				GameID:       model.NewGameID(),
				Board:        chess.InitialBoard(),
				Moves:        moveSeq,
				WhitePlayer:  inst.WhiteID,
				BlackPlayer:  inst.BlackID,
				ReplayCause:  inst.ReplayCause,
				ReplayResult: inst.ReplayResult,
				ReplayMode:   mode,
				InsertedTime: initialTimeGames.Add(time.Duration(gameIdx) * gameDurationOffset),
			})
			return err
		})
	}

	return eg.Wait()
}

type TournamentInsts struct {
	tournament   mutator.InsertTournamentParams
	participants []mutator.BatchInsertTournamentParticipantParams
	matches      []mutator.BatchInsertTournamentMatchParams
}

func generateTournaments() []TournamentInsts {
	defer perf.New().Log()

	// var tkeyUInt64 uint64

	var insts []TournamentInsts
	for range *tournamentsCount {
		// var tkey uuid.UUID
		// binary.LittleEndian.PutUint64(tkey[:], tkeyUInt64)
		// tkeyUInt64++
		// pgTkey := pgtype.UUID{Bytes: tkey, Valid: true}
		pgTkey := pgtype.UUID{Bytes: uuid.New(), Valid: true}

		createdBy := generateUserID(nil)
		status := model.TournamentInProgress
		rounds := rand.Intn(4) + 1

		var participants []mutator.BatchInsertTournamentParticipantParams
		var usedParticipants = map[int64]struct{}{}

		for i := range tournament.KnockoutParticipantsAtRound(rounds, 1) {
			var participantID int64
			if i == 0 {
				participantID = createdBy
			} else {
				participantID = generateUserID(usedParticipants)
			}
			joinedOn := time.Now().Add(-time.Minute*60 + time.Minute*time.Duration(i))
			participants = append(participants, mutator.BatchInsertTournamentParticipantParams{
				TournamentKey: pgTkey,
				UserID:        participantID,
				JoinedOn:      pgtype.Timestamptz{Time: joinedOn, Valid: true},
			})
			usedParticipants[participantID] = struct{}{}
		}

		var matches []mutator.BatchInsertTournamentMatchParams
		for i := 0; i+1 < len(participants); i += 2 {
			whiteID, blackID := participants[i].UserID, participants[i+1].UserID
			matches = append(matches, mutator.BatchInsertTournamentMatchParams{
				TournamentKey: pgTkey,
				GameID:        model.NewGameID().String(), // there are no games in the game store matching this at this point in time
				WhiteID:       whiteID,
				BlackID:       blackID,
				Round:         int32(1),
				CreatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
		}

		insts = append(insts, TournamentInsts{
			tournament: mutator.InsertTournamentParams{
				TournamentKey: pgTkey,
				Name:          fmt.Sprintf("%s's tournament", gofakeit.FirstName()),
				Rounds:        1,
				Status:        mutator.TournamentStatusEnum(status.String()),
				Mode:          mutator.ModeEnum(generateMode().String()),
				Ruleset:       mutator.TournamentRulesetEnum(model.TournamentKnockout.String()),
				CreatedBy:     createdBy,
				Countdown:     10,
				CreatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
			participants: participants,
			matches:      []mutator.BatchInsertTournamentMatchParams{},
		})
	}
	return insts
}

func seedTournaments(ctx context.Context, query database.QuerierMutator, paramsList []TournamentInsts) error {
	eg, egCtx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, Concurrency)

	for _, params := range paramsList {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			// participants and matches must be inserted after the tournament to maintain foreign key integrity
			if _, err := query.InsertTournament(egCtx, params.tournament); err != nil {
				return err
			}

			var errs []error

			query.BatchInsertTournamentParticipant(egCtx, params.participants).Exec(func(i int, err error) {
				errs = append(errs, err)
			})
			if err := errors.Join(errs...); err != nil {
				return err
			}

			query.BatchInsertTournamentMatch(egCtx, params.matches).Exec(func(i int, err error) {
				errs = append(errs, err)
			})
			return errors.Join(errs...)
		})
	}
	return nil
}
