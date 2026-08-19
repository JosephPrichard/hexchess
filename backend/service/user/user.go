package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"hexchess-svc/database/mutator"
	"hexchess-svc/model"
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"hexchess-svc/database"

	"hexchess-svc/utils/alog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/constraints"
	"golang.org/x/sync/errgroup"
)

type UserCRUDService struct {
	database.Database
}

func NewUserCRUDService(database database.Database) *UserCRUDService {
	return &UserCRUDService{Database: database}
}

type RankedUser struct {
	ID   int64
	Rank int64
}

var ErrUserNotFound = errors.New("user not found")
var ErrTakenUsername = errors.New("username already taken")

type Inst struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Country  string `json:"country"`
	JoinedOn time.Time
}

func (services *UserCRUDService) InsertUser(ctx context.Context, inst Inst) (model.User, error) {
	defer perf.WithContext(ctx).Log()

	if inst.JoinedOn.IsZero() {
		inst.JoinedOn = time.Now()
	}

	hash, err := hashPassword(inst.Password)
	if err != nil {
		return model.User{}, serrors.New("generate hash", err)
	}

	userRow, err := services.Mutator().InsertUser(ctx, mutator.InsertUserParams{
		Username: inst.Username,
		Country:  inst.Country,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
		JoinedOn: pgtype.Timestamptz{Valid: true, Time: inst.JoinedOn},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, ErrTakenUsername
		}
		return model.User{}, serrors.New("insert user to transactor", err)
	}

	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "created a new user", "user", user)
	return user, nil
}

func (services *UserCRUDService) BatchInsertUsers(ctx context.Context, insts []Inst) ([]model.User, error) {
	batches := make([]mutator.BatchInsertUserParams, len(insts))

	var hashEg errgroup.Group // no context propagation because jobs are non-cancellable

	for i, inst := range insts {
		if inst.JoinedOn.IsZero() {
			inst.JoinedOn = time.Now()
		}
		hashEg.Go(func() error {
			hash, err := hashPassword(inst.Password)
			if err != nil {
				return serrors.New("hash password for inst index", err, "index", i)
			}
			batch := mutator.BatchInsertUserParams{
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
	if err := hashEg.Wait(); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "batch inserting users", "insts", insts)

	var rows []mutator.BatchInsertUserRow
	var insertErrs []error
	services.Mutator().BatchInsertUser(ctx, batches).QueryRow(func(i int, row mutator.BatchInsertUserRow, err error) {
		if err == nil {
			rows = append(rows, row)
		} else {
			insertErrs = append(insertErrs, serrors.New("batch insert user", err, "index", i))
		}
	})
	err := errors.Join(insertErrs...)

	var users []model.User
	for _, row := range rows {
		users = append(users, model.User{
			ID:       row.ID,
			Username: row.Username,
			Country:  row.Country,
			Bio:      row.Bio,
			JoinedOn: row.JoinedOn.Time,
		})
	}

	alog.Log(ctx, "batch inserted users", err, "users", users)
	return users, err
}

type VerifiedUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Country  string `json:"country"`
}

const LoginAttemptsDivisor = 10
const LockoutDuration = time.Minute * 1

var ErrTooManyLoginAttempts = errors.New("too many login attempts")

func (services *UserCRUDService) VerifyUser(ctx context.Context, username string, inputPassword string) (VerifiedUser, error) {
	defer perf.WithContext(ctx).Log()

	var user VerifiedUser

	err := services.Database.ExecTx(ctx, database.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Non-Repeatable Read):
		// T1 is allowed to login due to valid login attempts L1 and but increases login attempt count from L1 to L2
		// In between reading L1 and updating from L1 to L2, another query updates the login attempts, so L1 will return an inconsistent value
		// Case 2 (Write Skew):
		// T1 and T2 attempt to login at the same time, with LoginAttempts-1 (L1) one less than an invalid threshold
		// T1 and T2 are both permitted to attempt to login since L1 is valid, even though only one login attempt is allowed
		// L2 will be L1+2 after update, even though it should not have permitted both attempts
		Isolation:    pgx.Serializable,
		ErrAllowlist: []error{ErrTooManyLoginAttempts, ErrUserNotFound},
		RetryCount:   3,
		QueryFn: func(ctx context.Context, txn pgx.Tx, query database.QuerierMutator) error {
			loginRow, err := query.SelectLoginByName(ctx, username)
			if database.IsErrNoRows(err) {
				return ErrUserNotFound
			} else if err != nil {
				return serrors.New("select user by login", err, "username", username)
			}

			isExceedAttempts := loginRow.LoginAttempts > 0 && loginRow.LoginAttempts%LoginAttemptsDivisor == 0
			nextLoginTime := loginRow.LastLoginAttempt.Time.Add(LockoutDuration)
			isLocked := isExceedAttempts && time.Now().Before(nextLoginTime)
			if isLocked {
				return ErrTooManyLoginAttempts
			}

			saltedPassword := inputPassword + loginRow.Salt

			err = bcrypt.CompareHashAndPassword([]byte(loginRow.Password), []byte(saltedPassword))
			if err != nil {
				if err := query.IncrLoginAttempts(ctx, loginRow.ID); err != nil {
					return serrors.New("increment user login attempts", err, "userID", loginRow.ID)
				}
				slog.ErrorContext(ctx, "failed to login, credentials are invalid", "username", username, "error", err)
				return ErrUserNotFound
			}

			if err := query.ResetLoginAttempts(ctx, loginRow.ID); err != nil {
				return serrors.New("reset user login attempts", err, "userID", loginRow.ID)
			}

			user = VerifiedUser{
				ID:       loginRow.ID,
				Username: loginRow.Username,
				Country:  loginRow.Country,
			}
			slog.InfoContext(ctx, "user login is valid", "user", user)
			return nil
		},
	})

	return user, err
}

type GoogleUserInst struct {
	Username string
	Country  string
	JoinedOn time.Time
}

func (services *UserCRUDService) SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error) {
	defer perf.WithContext(ctx).Log()

	var verifiedUser VerifiedUser
	var isCreated bool

	login, err := services.Querier().SelectByGoogleAccountID(ctx, pgtype.Text{String: googleAccountID, Valid: true})
	if database.IsErrNoRows(err) {
		isCreated = false
	} else if err != nil {
		return verifiedUser, serrors.New("select user by google account id", err, "googleAccountID", googleAccountID)
	} else {
		isCreated = true
	}

	if !isCreated {
		userRow, err := services.Mutator().InsertUser(ctx, mutator.InsertUserParams{
			Username:        googleInst.Username,
			Country:         googleInst.Country,
			JoinedOn:        pgtype.Timestamptz{Time: googleInst.JoinedOn, Valid: true},
			GoogleAccountID: pgtype.Text{String: googleAccountID, Valid: true},
		})
		if err != nil {
			return verifiedUser, serrors.New("insert google user", err, "googleAccountID", googleAccountID)
		}
		verifiedUser = VerifiedUser{
			ID:       userRow.ID,
			Username: userRow.Username,
			Country:  userRow.Country,
		}
		slog.InfoContext(ctx, "inserted a google user account", "inst", googleInst, "googleAccountID", googleAccountID)
	} else {
		verifiedUser = VerifiedUser{
			ID:       login.ID,
			Username: login.Username,
			Country:  login.Country,
		}
	}

	slog.InfoContext(ctx, "resolved verified user from googleAccountID", "user", verifiedUser, "googleAccountID", googleAccountID)
	return verifiedUser, nil
}

func ProbabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}

type UpdtUserParams struct {
	Username string
	Bio      string
	Country  string
}

func (services *UserCRUDService) UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (model.User, error) {
	defer perf.WithContext(ctx).Log()

	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return model.User{}, nil
	}

	userRow, err := services.Mutator().UpdateUser(ctx, mutator.UpdateUserParams{
		ID:       id,
		Username: database.OptString(updt.Username),
		Bio:      database.OptString(updt.Bio),
		Country:  database.OptString(updt.Country),
	})
	if err != nil {
		return model.User{}, serrors.New("update user", err, "userID", id)
	}

	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "updated user", "user", user)
	return user, err
}

func (services *UserCRUDService) UpdateUserPassword(ctx context.Context, id int64, newPassword string) error {
	defer perf.WithContext(ctx).Log()

	hash, err := hashPassword(newPassword)
	if err != nil {
		return serrors.New("hash password for user", err, "userID", id)
	}
	err = services.Mutator().UpdatePassword(ctx, mutator.UpdatePasswordParams{
		ID:       id,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
	})
	alog.Log(ctx, "updated password", err, "userID", id)
	return err
}

func (services *UserCRUDService) GetUserByID(ctx context.Context, id int64) (model.User, error) {
	defer perf.WithContext(ctx).Log()

	userRow, err := services.Querier().SelectUserByID(ctx, id)
	if database.IsErrNoRows(err) {
		return model.User{}, ErrUserNotFound
	} else if err != nil {
		return model.User{}, serrors.New("select user", err, "userID", id)
	}
	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "selected user", "userID", id, "user", user)
	return user, nil
}

func Average[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func (services *UserCRUDService) GetUserStats(ctx context.Context, id int64) (model.UserStats, error) {
	defer perf.WithContext(ctx).Log()

	modeEloRows, err := services.Querier().SelectUserElosByID(ctx, id)
	if err != nil {
		return model.UserStats{}, serrors.New("select user elos by id", err, "userID", id)
	}

	var stats model.UserStats
	if len(modeEloRows) == 0 {
		stats = model.UserStats{HighestElo: model.StartElo, AvgElo: model.StartElo}
	} else {
		stats = model.UserStats{HighestElo: math.SmallestNonzeroFloat64}
	}

	for _, row := range modeEloRows {
		mode := enum.Expect(row.Mode, model.GameModeEnums)
		modeStats := model.ModeStats{
			Mode:       mode,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			Winrate:    model.CalcUserWinrate(row.Wins, row.Losses, row.Draws),
			Elo:        row.Elo,
			HighestElo: row.HighestElo,
		}

		stats.TotalWins += modeStats.Wins
		stats.TotalLosses += modeStats.Losses
		stats.TotalDraws += modeStats.Draws

		stats.HighestElo = max(stats.HighestElo, modeStats.HighestElo)
		stats.AvgElo = Average(stats.AvgElo, len(stats.ModeStats), modeStats.Elo)
		stats.TotalWinrate = Average(stats.TotalWinrate, len(stats.ModeStats), modeStats.Winrate)

		stats.ModeStats = append(stats.ModeStats, modeStats)
	}

	slog.InfoContext(ctx, "selected user elos", "stats", stats)
	return stats, nil
}

func (services *UserCRUDService) SelectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error) {
	userRows, err := services.Querier().SelectUsersByIDs(ctx, ids)

	var users []model.User
	for _, row := range userRows {
		users = append(users, model.User{
			ID:       row.ID,
			Username: row.Username,
			Country:  row.Country,
		})
	}

	return users, err
}

type HashResult struct {
	Salt           string
	HashedPassword string
}

func hashPassword(password string) (HashResult, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return HashResult{}, serrors.New("generate salt", err)
	}
	salt := base64.StdEncoding.EncodeToString(saltBytes)

	saltedPassword := []byte(password + salt)

	hashed, err := bcrypt.GenerateFromPassword(saltedPassword, 12)
	if err != nil {
		return HashResult{}, serrors.New("hash password", err)
	}

	return HashResult{Salt: salt, HashedPassword: string(hashed)}, nil
}
