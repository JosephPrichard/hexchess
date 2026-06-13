package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/brianvoe/gofakeit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/chess"
	"hexchess-svc/cmd"
	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"
	"hexchess-svc/model"
	svc "hexchess-svc/service"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"
)

const trunateSql = `
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
	usersCount       = flag.Int("usersCount", 50, "number of users to seed")
	challengesCount  = flag.Int("challengesCount", 50, "number of challenges to seed")
	gameResultCount  = flag.Int("gameResultCount", 100, "number of game results to seed")
	tournamentsCount = flag.Int("tournamentsCount", 25, "number of tournaments to seed")
)

func main() {
	ctx := context.WithValue(context.Background(), logutil.Trace, "seed-databases-script")

	start := time.Now()

	shutdown := logutil.InitLoggers(logutil.LogConfig{})
	defer shutdown(ctx)

	cmd.InitEnv()

	dbURL := os.Getenv("DB_URL")
	rdbSorNodes := strings.Split(os.Getenv("REDIS_SOR_NODES"), ",")

	slog.InfoContext(ctx, "connecting to postgres db", "dbURL", dbURL)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logutil.FatalErr("create pool", err)
	}
	pdb := db.MakeDB(pool)
	defer pdb.Close()

	addrs := db.RedisAddrs{SorAddr: rdbSorNodes}
	slog.InfoContext(ctx, "connecting to redis db", "addrs", addrs)
	rdb := db.MakeRedis(addrs, nil)
	defer rdb.Close()

	_, err = pool.Exec(ctx, trunateSql)
	if err != nil {
		logutil.FatalErr("drop schema", err)
	}
	if err := rdb.Cache.FlushAll(ctx).Err(); err != nil {
		logutil.FatalErr("flush rdb", err)
	}

	services := svc.MakeHexchessServices(svc.SetupService{DB: pdb, Redis: rdb})

	userInsts := generateUserInsts()

	// root node in the foreign key hierarchy tree
	if _, err := services.BatchInsertUsers(ctx, userInsts); err != nil {
		logutil.FatalErr("insert users", err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := services.BatchInsertChallenges(egCtx, generateChallengeInsts()); err != nil {
			slog.ErrorContext(egCtx, "failed to seed challenges", "err", err)
		}
		return nil
	})
	eg.Go(func() error {
		return seedGameResults(egCtx, services, generateGameResults())
	})
	eg.Go(func() error {
		return seedTournaments(egCtx, pdb.Querier(), generateTournaments())
	})

	if err := eg.Wait(); err != nil {
		logutil.FatalErr("failed to seed user dependent rows", err)
	}

	// syncs the stat updates written in the game results into the leaderboard.
	if err := services.SyncLeaderboard(ctx); err != nil {
		logutil.FatalErr("jobs leaderboard", err)
	}

	log.Printf("finished seeding databases: %v", time.Since(start))
}

func generateUserInsts() []svc.UserInst {
	var insts []svc.UserInst
	for range *usersCount {
		insts = append(insts, svc.UserInst{
			Username: gofakeit.Username(),
			Password: "password1",
			Country:  "us",
			JoinedOn: time.Now(),
		})
	}
	return insts
}

func generateUserID(uselistedIDs map[int64]struct{}) int64 {
	for range 10 {
		userID := rand.Int63n(int64(*usersCount)) + 1
		if uselistedIDs == nil {
			return userID
		}
		if _, used := uselistedIDs[userID]; !used {
			return userID
		}
	}
	logutil.Fatal("failed to generate user id (all are uselisted)")
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
	var insts []svc.ChallengeInst
	for range *challengesCount {
		challengerID, challengeeID := generateUserIDPairs()
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
	timeAt := time.Now().Add(-1 * time.Hour * 24 * 100)

	a := insts
	rand.Shuffle(len(a), func(i, j int) {
		a[i], a[j] = a[j], a[i]
	})

	eg, egCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 64)

	for gameIdx, inst := range insts {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			mode := inst.ReplayMode

			moveSeq, err := svc.RandomMoveHistSeq(mode, chess.MakeStartGame(), 10, 30)
			if err != nil {
				return fmt.Errorf("generate random move seq: %w", err)
			}
			moveHistBlob, err := chess.MarshalMoveHistory(chess.InitialBoard(), moveSeq)
			if err != nil {
				return fmt.Errorf("marshal move history to s3: %w", err)
			}

			changeSet, err := services.InsertGameResult(egCtx, svc.GameResult{
				GameID:       svc.MakeGameID(),
				WhiteID:      inst.WhiteID,
				BlackID:      inst.BlackID,
				ReplayCause:  inst.ReplayCause,
				ReplayResult: inst.ReplayResult,
				ReplayMode:   mode,
				InsertedTime: timeAt.Add(time.Duration(gameIdx) * time.Hour * 24),
				TurnCount:    len(moveSeq),
			})
			if err != nil {
				return fmt.Errorf("insert game result: %w", err)
			}
			// note: don't forget to insert the move history - it exists outside of the game result tx
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
	var insts []TournamentInsts
	for range *tournamentsCount {
		tkey := pgtype.UUID{Bytes: uuid.New(), Valid: true}
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
				TournamentKey: tkey,
				UserID:        participantID,
				JoinedOn:      pgtype.Timestamptz{Time: joinedOn, Valid: true},
			})
			usedParticipants[participantID] = struct{}{}
		}

		var matches []sqlc.BatchInsertTournamentMatchParams
		for i := 0; i+1 < len(participants); i += 2 {
			whiteID, blackID := participants[i].UserID, participants[i+1].UserID
			matches = append(matches, sqlc.BatchInsertTournamentMatchParams{
				TournamentKey: tkey,
				GameID:        svc.MakeGameID(), // there are no games the game store matching this at this point in time
				WhiteID:       whiteID,
				BlackID:       blackID,
				Round:         int32(1),
				CreatedOn:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
		}

		insts = append(insts, TournamentInsts{
			tournament: sqlc.InsertTournamentParams{
				TournamentKey: tkey,
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

	sem := make(chan struct{}, 64)

	for _, params := range paramsList {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			if _, err := querier.InsertTournament(egCtx, params.tournament); err != nil {
				return err
			}

			var participantErrs []error
			querier.BatchInsertTournamentParticipant(egCtx, params.participants).Exec(func(i int, err error) {
				participantErrs = append(participantErrs, err)
			})
			if err := errors.Join(participantErrs...); err != nil {
				return err
			}

			var matchErrs []error
			querier.BatchInsertTournamentMatch(egCtx, params.matches).Exec(func(i int, err error) {
				matchErrs = append(matchErrs, err)
			})
			return errors.Join(matchErrs...)
		})
	}
	return nil
}
