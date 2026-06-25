package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/config"
	"hexchess-svc/lib/dotenv"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/lib/perf"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

const truncateSql = `
	TRUNCATE TABLE 
		users, 
		replays, 
		challenges, 
		event_queue, 
		event_keys, 
		tournaments, 
		tournament_matches, 
		tournament_participants, 
		user_mode_elos, 
		replay_move_histories
	RESTART IDENTITY
	CASCADE;`

var (
	usersCount             = flag.Int("usersCount", 1000, "number of users to seed")
	challengesCount        = flag.Int("challengesCount", 100, "number of challenges to seed")
	gameResultCount        = flag.Int("gameResultCount", 5000, "number of game results to seed")
	tournamentsCount       = flag.Int("tournamentsCount", 1000, "number of tournaments to seed")
	deterministicUsernames = flag.Bool("deterministicUsernames", true, "whether usernames follow the pattern 'User0', 'User1', etc. or are random")
	initialTimeGamesRaw    = flag.String("initialTimeGames", "2025-01-01", "the oldest date at which generated game results start from")
	gameDurationOffsetRaw  = flag.String("gameDurationOffset", "24h", "the offset between the time each consecutive game is played on (e.g. 1h, 5m)")

	initialTimeGames   = time.Time{}
	gameDurationOffset = time.Duration(0)
)

const (
	ServiceName = "hexchess-seed"
	Concurrency = 64
)

func main() {
	// parse: input data parameters to generate seeded data backend
	t, err := time.Parse(time.DateOnly, *initialTimeGamesRaw)
	if err != nil {
		logutil.Fatal("failed to parse date", err, "initialTimeGameResults", *initialTimeGamesRaw)
	}
	initialTimeGames = t

	d, err := time.ParseDuration(*gameDurationOffsetRaw)
	if err != nil {
		logutil.Fatal("failed to parse duration", err, "gameDurationOffset", *gameDurationOffsetRaw)
	}
	gameDurationOffset = d

	// validation: check that it is reasonable to generate this number of challenges
	maxChallengePermutations := *usersCount * (*usersCount - 1)
	maxChallengesCount := maxChallengePermutations / 10
	if *challengesCount > maxChallengesCount {
		log.Fatalf("challenges count is too large, must be at most %d", maxChallengesCount)
	}

	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	dotenv.Load()

	dbURL := os.Getenv("DB_URL")
	profile := config.ParseProfile(os.Getenv("ACTIVE_PROFILE"))
	awsRegion := os.Getenv("AWS_REGION")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")
	rdbSorUsername := os.Getenv("REDIS_SOR_USERNAME")
	rdbSorClusterName := os.Getenv("REDIS_SOR_CLUSTER_NAME")
	oltpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	start := time.Now()

	shutdown := logutil.InitLoggers(ServiceName, oltpEndpoint, profile)
	defer shutdown()

	pool := db.NewPgPool(ctx, db.PgPoolConfig{
		Dsn:           dbURL,
		ActiveProfile: profile,
		Region:        awsRegion,
	})
	pdb := db.NewPostgresDB(pool)
	defer pdb.Close()

	rdb := db.NewRedis(ctx, db.RedisConfig{
		SorAddr:        rdbSorNodes,
		SorUsername:    rdbSorUsername,
		SorClusterName: rdbSorClusterName,
		ActiveProfile:  profile,
	})
	defer rdb.Close()

	if _, err := pool.Exec(ctx, truncateSql); err != nil {
		logutil.Fatal("drop schema", err)
	}
	if err := rdb.Primary.FlushAll(ctx).Err(); err != nil {
		logutil.Fatal("flush rdb", err)
	}

	var usersDuration time.Duration
	var challengesDuration time.Duration
	var resultsDuration time.Duration
	var tournamentsDuration time.Duration

	services := svc.NewHexchessServices(svc.SetupService{DB: pdb, Redis: rdb})

	// root node in the foreign key hierarchy tree
	usersStart := time.Now()
	if _, err := services.BatchInsertUsers(ctx, generateUserInsts()); err != nil {
		logutil.Fatal("insert users", err)
	}
	usersDuration = time.Since(usersStart)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer perf.New().Duration(&challengesDuration)
		return services.BatchInsertChallenges(egCtx, generateChallengeInsts())
	})
	eg.Go(func() error {
		defer perf.New().Duration(&resultsDuration)
		return seedGameResults(egCtx, services, generateGameResults())
	})
	eg.Go(func() error {
		defer perf.New().Duration(&tournamentsDuration)
		return seedTournaments(egCtx, pdb.Querier(), generateTournaments())
	})

	if err := eg.Wait(); err != nil {
		logutil.Fatal("failed to seed user dependent rows", err)
	}

	// syncs the stat updates written in the game results into the leaderboard.
	if err := services.SyncLeaderboard(ctx); err != nil {
		logutil.Fatal("jobs leaderboard", err)
	}

	slog.Info("finished seeding databases",
		"timeTaken", time.Since(start).String(),
		"usersDuration", usersDuration.String(),
		"challengesDuration", challengesDuration.String(),
		"gameResultsDuration", resultsDuration.String(),
		"tournamentsDuration", tournamentsDuration.String())
}

func generateUserInsts() []svc.UserInst {
	var insts []svc.UserInst
	for i := range *usersCount {
		var username string
		if *deterministicUsernames {
			username = fmt.Sprintf("User%d", i)
		} else {
			username = gofakeit.Username()
		}

		insts = append(insts, svc.UserInst{
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
	logutil.Fatal("generate user id (all are uselisted)", nil)
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

func generateChallengeInsts() []svc.ChallengeInst {
	hashChallengeKey := func(challengerID, challengeeID int64) string {
		return fmt.Sprintf("%d,%d", challengerID, challengeeID)
	}

	var insts []svc.ChallengeInst
	for range *challengesCount {
		// generate two challenges that are unique, this is done by retrying if a duplicate is found.
		// note: we assume the number of users is large enough to avoid duplicates

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

		insts = append(insts, svc.ChallengeInst{
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

func seedGameResults(ctx context.Context, services *svc.HexchessServices, insts []GameResultInsts) error {
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

			moveSeq, err := svc.RandomMoveHistSeq(mode, 10, 30, -1)
			if err != nil {
				return fmt.Errorf("generate random move seq: %w", err)
			}
			moveHistBlob, err := chess.MarshalMoveHistory(chess.InitialBoard(), moveSeq)
			if err != nil {
				return fmt.Errorf("marshal move history to s3: %w", err)
			}

			changeSet, err := services.InsertGameResult(egCtx, svc.GameResult{
				GameID:       model.NewGameID(),
				WhiteID:      inst.WhiteID,
				BlackID:      inst.BlackID,
				ReplayCause:  inst.ReplayCause,
				ReplayResult: inst.ReplayResult,
				ReplayMode:   mode,
				InsertedTime: initialTimeGames.Add(time.Duration(gameIdx) * gameDurationOffset),
				TurnCount:    len(moveSeq),
			})
			if err != nil {
				return fmt.Errorf("insert game result: %w", err)
			}
			// note: remember to insert the move history - it exists outside the game result tx
			if err = services.UpsertReplayMoveHistories(ctx, changeSet.ReplayID, moveHistBlob); err != nil {
				return fmt.Errorf("insert replay move histories: %w", err)
			}
			return nil
		})
	}

	return eg.Wait()
}

type TournamentInsts struct {
	tournament   sqlc.InsertTournamentParams
	participants []sqlc.BatchInsertTournamentParticipantParams
	matches      []sqlc.BatchInsertTournamentMatchParams
}

func generateTournaments() []TournamentInsts {
	var tkeyUInt64 uint64

	var insts []TournamentInsts
	for range *tournamentsCount {
		var tkey uuid.UUID
		binary.LittleEndian.PutUint64(tkey[:], tkeyUInt64)
		tkeyUInt64++
		pgtkey := pgtype.UUID{Bytes: tkey, Valid: true}

		createdBy := generateUserID(nil)
		status := model.TournamentInProgress
		rounds := rand.Intn(4) + 1

		var participants []sqlc.BatchInsertTournamentParticipantParams
		var usedParticipants = map[int64]struct{}{}

		for i := range svc.KnockoutParticipantsAtRound(rounds, 1) {
			var participantID int64
			if i == 0 {
				participantID = createdBy
			} else {
				participantID = generateUserID(usedParticipants)
			}
			joinedOn := time.Now().Add(-time.Minute*60 + time.Minute*time.Duration(i))
			participants = append(participants, sqlc.BatchInsertTournamentParticipantParams{
				TournamentKey: pgtkey,
				UserID:        participantID,
				JoinedOn:      pgtype.Timestamptz{Time: joinedOn, Valid: true},
			})
			usedParticipants[participantID] = struct{}{}
		}

		var matches []sqlc.BatchInsertTournamentMatchParams
		for i := 0; i+1 < len(participants); i += 2 {
			whiteID, blackID := participants[i].UserID, participants[i+1].UserID
			matches = append(matches, sqlc.BatchInsertTournamentMatchParams{
				TournamentKey: pgtkey,
				GameID:        model.NewGameID().String(), // there are no games in the game store matching this at this point in time
				WhiteID:       whiteID,
				BlackID:       blackID,
				Round:         int32(1),
				CreatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
		}

		insts = append(insts, TournamentInsts{
			tournament: sqlc.InsertTournamentParams{
				TournamentKey: pgtkey,
				Name:          fmt.Sprintf("%s's tournament", gofakeit.FirstName()),
				Rounds:        1,
				Status:        sqlc.TournamentStatusEnum(status.String()),
				Mode:          sqlc.ModeEnum(generateMode().String()),
				Ruleset:       sqlc.TournamentRulesetEnum(model.TournamentKnockout.String()),
				CreatedBy:     createdBy,
				Countdown:     10,
				CreatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
			participants: participants,
			matches:      []sqlc.BatchInsertTournamentMatchParams{},
		})
	}
	return insts
}

func seedTournaments(ctx context.Context, querier sqlc.Querier, paramsList []TournamentInsts) error {
	eg, egCtx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, Concurrency)

	for _, params := range paramsList {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			// participants and matches must be inserted after the tournament to maintain foreign key integrity
			if _, err := querier.InsertTournament(egCtx, params.tournament); err != nil {
				return err
			}

			var errs []error

			querier.BatchInsertTournamentParticipant(egCtx, params.participants).Exec(func(i int, err error) {
				errs = append(errs, err)
			})
			if err := errors.Join(errs...); err != nil {
				return err
			}

			querier.BatchInsertTournamentMatch(egCtx, params.matches).Exec(func(i int, err error) {
				errs = append(errs, err)
			})
			return errors.Join(errs...)
		})
	}
	return nil
}
