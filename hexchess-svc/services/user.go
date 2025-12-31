package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hexchess-svc/db"
	"hexchess-svc/pkg/logutil"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/constraints"
	"golang.org/x/sync/errgroup"
)

type UserEntity struct {
	ID       int64     `json:"id"`
	Username string    `json:"username"`
	Country  string    `json:"country"`
	Bio      string    `json:"bio"`
	JoinedOn time.Time `json:"joinedOn"`
}

const StartElo float64 = 1000
const DefaultCountry = "un"

func defaultElo(elo pgtype.Float8) float64 {
	if elo.Valid {
		return elo.Float64
	} else {
		return StartElo
	}
}

type RankedUser struct {
	ID   int64
	Rank int64
}

var ErrUserNotFound = errors.New("user not found")
var ErrTakenUsername = errors.New("username already taken")

type HashResult struct {
	Salt           string
	HashedPassword string
}

func hashPassword(password string) (HashResult, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return HashResult{}, fmt.Errorf("generate salt: %w", err)
	}
	salt := base64.StdEncoding.EncodeToString(saltBytes)

	saltedPassword := []byte(password + salt)

	hashed, err := bcrypt.GenerateFromPassword(saltedPassword, 12)
	if err != nil {
		return HashResult{}, fmt.Errorf("hash password: %w", err)
	}

	return HashResult{Salt: salt, HashedPassword: string(hashed)}, nil
}

func calcUserWinrate(wins int32, losses int32, draws int32) int64 {
	wr := float64(0)
	total := wins + losses + draws
	if total > 0 {
		wr = float64(wins) / float64(total) * 100.0
	}
	return int64(wr)
}

func mapUserFromRow(row db.SelectUserByIDRow) UserEntity {
	return UserEntity{
		ID:       row.ID,
		Username: row.Username,
		Country:  row.Country,
		Bio:      row.Bio,
		JoinedOn: row.JoinedOn.Time,
	}
}

type UserInst struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Country  string `json:"country"`
	JoinedOn time.Time
}

func InsertUser(ctx context.Context, query *db.Queries, inst UserInst) (UserEntity, error) {
	if inst.JoinedOn.IsZero() {
		inst.JoinedOn = time.Now()
	}

	var u UserEntity
	hash, err := hashPassword(inst.Password)
	if err != nil {
		return u, fmt.Errorf("generate hash: %w", err)
	}

	row, err := query.InsertUser(ctx, db.InsertUserParams{
		Username: inst.Username,
		Country:  inst.Country,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
		JoinedOn: pgtype.Timestamptz{Valid: true, Time: inst.JoinedOn},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return u, ErrTakenUsername
		}
		return u, fmt.Errorf("insert user to db: %w", err)
	}

	u = mapUserFromRow(db.SelectUserByIDRow(row))
	slog.InfoContext(ctx, "created a new user", "user", u)
	return u, nil
}

func BatchInsertUsers(ctx context.Context, query *db.Queries, insts []UserInst) ([]UserEntity, error) {
	batches := make([]db.BatchInsertUserParams, len(insts))

	var eg errgroup.Group
	for i, inst := range insts {
		if inst.JoinedOn.IsZero() {
			inst.JoinedOn = time.Now()
		}
		eg.Go(func() error {
			hash, err := hashPassword(inst.Password)
			if err != nil {
				return fmt.Errorf("hash password for inst index %d: %w", i, err)
			}
			batch := db.BatchInsertUserParams{
				Username: inst.Username,
				Country:  inst.Country,
				Password: hash.HashedPassword,
				Salt:     hash.Salt,
				JoinedOn: pgtype.Timestamptz{Valid: true, Time: inst.JoinedOn},
			}
			batches[i] = batch
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "failed to batch insert users", "insts", insts, "err", err)
		return nil, err
	}

	var users []UserEntity
	var errs []error

	query.BatchInsertUser(ctx, batches).QueryRow(func(i int, row db.BatchInsertUserRow, err error) {
		if err != nil {
			errs = append(errs, err)
		} else {
			users = append(users, mapUserFromRow(db.SelectUserByIDRow(row)))
		}
	})

	logutil.DynLog(ctx, "batch inserted user", errors.Join(errs...), "insts", insts, "users", users)
	return users, nil
}

type VerifiedUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Country  string `json:"country"`
}

func VerifyUserTx(ctx context.Context, pdb *db.Postgres, username string, inputPassword string) (VerifiedUser, error) {
	return db.RunInTx(ctx, pdb,
		[]error{ErrTooManyLoginAttempts, ErrUserNotFound},
		func(ctx context.Context, query *db.Queries) (VerifiedUser, error) {
			return verifyUser(ctx, query, username, inputPassword)
		},
	)
}

const LoginAttemptsDivisor = 10
const LockoutDuration = time.Minute * 1

var ErrTooManyLoginAttempts = errors.New("too many login attempts")

func verifyUser(ctx context.Context, query *db.Queries, username string, inputPassword string) (VerifiedUser, error) {
	var u VerifiedUser

	login, err := query.SelectLoginByName(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return u, ErrUserNotFound
		}
		return u, fmt.Errorf("select user '%s' by login: %w", username, err)
	}

	isExceedAttempts := login.LoginAttempts > 0 && login.LoginAttempts%LoginAttemptsDivisor == 0
	nextLoginTime := login.LastLoginAttempt.Time.Add(LockoutDuration)
	isLocked := isExceedAttempts && time.Now().Before(nextLoginTime)
	if isLocked {
		return u, ErrTooManyLoginAttempts
	}

	saltedPassword := inputPassword + login.Salt
	loginErr := bcrypt.CompareHashAndPassword([]byte(login.Password), []byte(saltedPassword))

	if loginErr != nil {
		if err := query.IncrLoginAttempts(ctx, login.ID); err != nil {
			return u, fmt.Errorf("incr user %d login attempts: %w", login.ID, err)
		}
		slog.ErrorContext(ctx, "failed to user login is invalid", "username", username, "err", loginErr)
		return u, ErrUserNotFound
	}

	if err := query.ResetLoginAttempts(ctx, login.ID); err != nil {
		return u, fmt.Errorf("reset user %d login attempts: %w", login.ID, err)
	}
	u = VerifiedUser{
		ID:       login.ID,
		Username: login.Username,
		Country:  login.Country,
	}
	slog.InfoContext(ctx, "user login is valid", "user", u)
	return u, nil
}

type GoogleUserInst struct {
	Username string
	Country  string
	JoinedOn time.Time
}

func SelectOrInsertGoogleUser(ctx context.Context, query *db.Queries, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error) {
	var u VerifiedUser
	var isCreated bool

	login, err := query.SelectByGoogleAccountID(ctx, pgtype.Text{String: googleAccountID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		isCreated = false
	} else if err != nil {
		return u, fmt.Errorf("select user '%s' by google account id: %w", googleAccountID, err)
	} else {
		isCreated = true
	}

	if !isCreated {
		row, err := query.InsertUser(ctx, db.InsertUserParams{
			Username:        googleInst.Username,
			Country:         googleInst.Country,
			JoinedOn:        pgtype.Timestamptz{Time: googleInst.JoinedOn, Valid: true},
			GoogleAccountID: pgtype.Text{String: googleAccountID, Valid: true},
		})
		if err != nil {
			return u, fmt.Errorf("insert google user '%s': %w", googleAccountID, err)
		}
		u = VerifiedUser{
			ID:       row.ID,
			Username: row.Username,
			Country:  row.Country,
		}
		slog.InfoContext(ctx, "inserted a google user account", "inst", googleInst, "googleAccountID", googleAccountID)
	} else {
		u = VerifiedUser{
			ID:       login.ID,
			Username: login.Username,
			Country:  login.Country,
		}
	}

	slog.InfoContext(ctx, "resolved verified user from googleAccountID", "user", u, "googleAccountID", googleAccountID)
	return u, nil
}

func ProbabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}

type UpdtUserParams struct {
	Username string
	Bio      string
	Country  string
}

func UpdateUser(ctx context.Context, query *db.Queries, id int64, updt UpdtUserParams) (UserEntity, error) {
	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return UserEntity{}, nil
	}

	row, err := query.UpdateUser(ctx, db.UpdateUserParams{
		ID:       id,
		Username: pgtype.Text{Valid: updt.Username != "", String: updt.Username},
		Bio:      pgtype.Text{Valid: updt.Bio != "", String: updt.Bio},
		Country:  pgtype.Text{Valid: updt.Country != "", String: updt.Country},
	})

	user := mapUserFromRow(db.SelectUserByIDRow(row))
	logutil.DynLog(ctx, "updated user", err, "user", user)
	return user, err
}

func UpdateUserPassword(ctx context.Context, query *db.Queries, id int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password for user %d: %w", id, err)
	}
	err = query.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:       id,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
	})
	logutil.DynLog(ctx, "updated password", err, "id", id)
	return err
}

func GetUserByID(ctx context.Context, query *db.Queries, id int64) (UserEntity, error) {
	row, err := query.SelectUserByID(ctx, id)
	if err != nil {
		return UserEntity{}, fmt.Errorf("select user %d: %w", id, err)
	}
	user := mapUserFromRow(row)
	slog.InfoContext(ctx, "selected user", "id", id, "user", user)
	return user, nil
}

type ModeStatsEntity struct {
	Mode       string  `json:"mode"`
	Rank       int64   `json:"rank"`
	Wins       int32   `json:"wins"`
	Losses     int32   `json:"losses"`
	Draws      int32   `json:"draws"`
	Winrate    int64   `json:"winrate"`
	Elo        float64 `json:"elo"`
	HighestElo float64 `json:"highestElo"`
}

type UserStatsEntity struct {
	TotalWins    int32             `json:"totalWins"`
	TotalLosses  int32             `json:"totalLosses"`
	TotalDraws   int32             `json:"totalDraws"`
	AvgElo       float64           `json:"avgElo"`     // average elo of all other modes
	HighestElo   float64           `json:"highestElo"` // the absolute highest elo
	TotalWinrate int64             `json:"totalWinrate"`
	ModeStats    []ModeStatsEntity `json:"modeStats"`
}

func avg[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func GetUserStats(ctx context.Context, query *db.Queries, id int64) (UserStatsEntity, error) {
	stats := UserStatsEntity{HighestElo: math.SmallestNonzeroFloat64}

	rows, err := query.SelectUserElosById(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("select user %d elos by id: %w", id, err)
	}

	for _, row := range rows {
		modeStats := ModeStatsEntity{
			Mode:       string(row.Mode),
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			Winrate:    calcUserWinrate(row.Wins, row.Losses, row.Draws),
			Elo:        row.Elo,
			HighestElo: row.HighestElo,
		}

		stats.TotalWins += modeStats.Wins
		stats.TotalLosses += modeStats.Losses
		stats.TotalDraws += modeStats.Draws

		stats.HighestElo = max(stats.HighestElo, modeStats.HighestElo)
		stats.AvgElo = avg(stats.AvgElo, len(stats.ModeStats), modeStats.Elo)
		stats.TotalWinrate = avg(stats.TotalWinrate, len(stats.ModeStats), modeStats.Winrate)

		stats.ModeStats = append(stats.ModeStats, modeStats)
	}

	slog.InfoContext(ctx, "selected user elos", "stats", stats)
	return stats, nil
}
