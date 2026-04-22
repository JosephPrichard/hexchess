package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hexchess-svc/domain"
	"hexchess-svc/util/enum"
	"hexchess-svc/util/errutil"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/util/logutil"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/constraints"
	"golang.org/x/sync/errgroup"
)

type RankedUser struct {
	ID   int64
	Rank int64
}

var ErrUserNotFound = errors.New("user not found")
var ErrTakenUsername = errors.New("username already taken")

type UserInst struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Country  string `json:"country"`
	JoinedOn time.Time
}

func (svc *HexchessServices) InsertUser(ctx context.Context, inst UserInst) (domain.User, error) {
	if inst.JoinedOn.IsZero() {
		inst.JoinedOn = time.Now()
	}

	hash, err := hashPassword(inst.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("generate hash: %w", err)
	}

	userRow, err := svc.querier.InsertUser(ctx, sqlc.InsertUserParams{
		Username: inst.Username,
		Country:  inst.Country,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
		JoinedOn: pgtype.Timestamptz{Valid: true, Time: inst.JoinedOn},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrTakenUsername
		}
		return domain.User{}, fmt.Errorf("insert user to db: %w", err)
	}

	user := domain.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "created a new user", "user", user)
	return user, nil
}

func (svc *HexchessServices) BatchInsertUsers(ctx context.Context, insts []UserInst) ([]domain.User, error) {
	batches := make([]sqlc.BatchInsertUserParams, len(insts))

	var hashEg errgroup.Group // no context propagation because jobs are non-cancellable

	for i, inst := range insts {
		if inst.JoinedOn.IsZero() {
			inst.JoinedOn = time.Now()
		}
		hashEg.Go(func() error {
			hash, err := hashPassword(inst.Password)
			if err != nil {
				return fmt.Errorf("hash password for inst index %d: %w", i, err)
			}
			batch := sqlc.BatchInsertUserParams{
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

	var users []domain.User
	var insertErrs []error

	svc.querier.BatchInsertUser(ctx, batches).QueryRow(func(i int, row sqlc.BatchInsertUserRow, err error) {
		if err == nil {
			users = append(users, domain.User{
				ID:       row.ID,
				Username: row.Username,
				Country:  row.Country,
				Bio:      row.Bio,
				JoinedOn: row.JoinedOn.Time,
			})
		} else {
			insertErrs = append(insertErrs, err)
		}
	})

	err := errors.Join(insertErrs...)

	logutil.DynLog(ctx, "batch inserted user", err, "insts", insts, "users", users)
	return users, err
}

type VerifiedUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Country  string `json:"country"`
}

func (svc *HexchessServices) VerifyUserTx(ctx context.Context, username string, inputPassword string) (VerifiedUser, error) {
	var user VerifiedUser

	err := svc.db.ExecTx(ctx, db.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Non-Repeatable Read):
		// T1 is allowed to login due to valid login attempts L1 and but increases login attempt count from L1 to L2
		// In between reading L1 and updating from L1 to L2, another query updates the login attempts, so L1 will return an inconsistent value
		// Case 2 (Write Skew):
		// T1 and T2 attempt to login at the same time, with LoginAttempts-1 (L1) one less than an invalid threshold
		// T1 and T2 are both permitted to attempt to login since L1 is valid, even though only one login attempt is allowed
		// L2 will be L1+2 after update, even though it should not have permitted both attempts
		Isolation: pgx.Serializable,
		QueryFn: func(ctx context.Context, query sqlc.Querier) (err error) {
			user, err = verifyUser(ctx, query, username, inputPassword)
			return err
		},
		ErrAllowlist: []error{ErrTooManyLoginAttempts, ErrUserNotFound},
		RetryCount:   3,
	})

	return user, err
}

const LoginAttemptsDivisor = 10
const LockoutDuration = time.Minute * 1

var ErrTooManyLoginAttempts = errors.New("too many login attempts")

func verifyUser(ctx context.Context, querier sqlc.Querier, username string, inputPassword string) (u VerifiedUser, err error) {
	loginRow, err := querier.SelectLoginByName(ctx, username)
	if IsErrNoRows(err) {
		return u, ErrUserNotFound
	} else if err != nil {
		return u, fmt.Errorf("select user [%s] by login: %w", username, err)
	}

	isExceedAttempts := loginRow.LoginAttempts > 0 && loginRow.LoginAttempts%LoginAttemptsDivisor == 0
	nextLoginTime := loginRow.LastLoginAttempt.Time.Add(LockoutDuration)
	isLocked := isExceedAttempts && time.Now().Before(nextLoginTime)
	if isLocked {
		return u, ErrTooManyLoginAttempts
	}

	saltedPassword := inputPassword + loginRow.Salt
	loginErr := bcrypt.CompareHashAndPassword([]byte(loginRow.Password), []byte(saltedPassword))

	if loginErr != nil {
		if err := querier.IncrLoginAttempts(ctx, loginRow.ID); err != nil {
			return u, fmt.Errorf("increment user [%d] login attempts: %w", loginRow.ID, err)
		}
		slog.ErrorContext(ctx, "failed to login, credentials are invalid", "username", username, "err", loginErr)
		return u, ErrUserNotFound
	}

	if err := querier.ResetLoginAttempts(ctx, loginRow.ID); err != nil {
		return u, fmt.Errorf("reset user [%d] login attempts: %w", loginRow.ID, err)
	}

	user := VerifiedUser{
		ID:       loginRow.ID,
		Username: loginRow.Username,
		Country:  loginRow.Country,
	}
	slog.InfoContext(ctx, "user login is valid", "user", user)
	return user, nil
}

type GoogleUserInst struct {
	Username string
	Country  string
	JoinedOn time.Time
}

func (svc *HexchessServices) SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error) {
	var verifiedUser VerifiedUser
	var isCreated bool

	login, err := svc.querier.SelectByGoogleAccountID(ctx, pgtype.Text{String: googleAccountID, Valid: true})
	if IsErrNoRows(err) {
		isCreated = false
	} else if err != nil {
		return verifiedUser, fmt.Errorf("select user [%s] by google account id: %w", googleAccountID, err)
	} else {
		isCreated = true
	}

	if !isCreated {
		userRow, err := svc.querier.InsertUser(ctx, sqlc.InsertUserParams{
			Username:        googleInst.Username,
			Country:         googleInst.Country,
			JoinedOn:        pgtype.Timestamptz{Time: googleInst.JoinedOn, Valid: true},
			GoogleAccountID: pgtype.Text{String: googleAccountID, Valid: true},
		})
		if err != nil {
			return verifiedUser, fmt.Errorf("insert google user [%s]: %w", googleAccountID, err)
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

func (svc *HexchessServices) UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (domain.User, error) {
	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return domain.User{}, nil
	}

	userRow, err := svc.querier.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:       id,
		Username: pgtype.Text{Valid: updt.Username != "", String: updt.Username},
		Bio:      pgtype.Text{Valid: updt.Bio != "", String: updt.Bio},
		Country:  pgtype.Text{Valid: updt.Country != "", String: updt.Country},
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("update user %d: %w", id, err)
	}

	user := domain.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "updated user", "user", user)
	return user, err
}

func (svc *HexchessServices) UpdateUserPassword(ctx context.Context, id int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password for user [%d]: %w", id, err)
	}
	err = svc.querier.UpdatePassword(ctx, sqlc.UpdatePasswordParams{
		ID:       id,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
	})
	logutil.DynLog(ctx, "updated password", err, "id", id)
	return err
}

func (svc *HexchessServices) GetUserByID(ctx context.Context, id int64) (domain.User, error) {
	userRow, err := svc.querier.SelectUserByID(ctx, id)
	if err != nil {
		if IsErrNoRows(err) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("select user [%d]: %w", id, err)
	}
	user := domain.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "selected user", "id", id, "user", user)
	return user, nil
}

func avg[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func (svc *HexchessServices) GetUserStats(ctx context.Context, id int64) (domain.UserStats, error) {
	modeEloRows, err := svc.querier.SelectUserElosByID(ctx, id)
	if err != nil {
		return domain.UserStats{}, fmt.Errorf("select user [%d] elos by id: %w", id, err)
	}

	var stats domain.UserStats
	if len(modeEloRows) == 0 {
		stats = domain.UserStats{HighestElo: domain.StartElo, AvgElo: domain.StartElo}
	} else {
		stats = domain.UserStats{HighestElo: math.SmallestNonzeroFloat64}
	}

	for _, row := range modeEloRows {
		mode := enum.Expect(row.Mode, domain.GameModeEnums)
		modeStats := domain.ModeStats{
			Mode:       mode,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			Winrate:    domain.CalcUserWinrate(row.Wins, row.Losses, row.Draws),
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

type FullUser struct {
	User       domain.User         `json:"user"`
	Stats      domain.UserStats    `json:"stats"`
	ReplayList []domain.FullReplay `json:"replayList"`
}

func (svc *HexchessServices) GetFullUser(ctx context.Context, userID int64, perPage int32, withReplays bool) (FullUser, error) {
	var user domain.User
	var stats domain.UserStats
	var replayList []domain.FullReplay
	var lbRanks map[string]LbRank

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		user, err = svc.GetUserByID(egCtx, userID)
		return errutil.Guardf(err, "get user %d", userID)
	})
	eg.Go(func() (err error) {
		stats, err = svc.GetUserStats(egCtx, userID)
		return errutil.Guardf(err, "get user %d stats", userID)
	})
	eg.Go(func() (err error) {
		lbRanks, err = svc.GetUserLeaderboardRanks(egCtx, userID, domain.GameModeEnums)
		return errutil.Guardf(err, "get user %d leaderboard ranks", userID)
	})
	if withReplays {
		eg.Go(func() (err error) {
			replayList, err = svc.GetUserReplays(egCtx, userID, -1, perPage)
			return errutil.Guardf(err, "get user %d replays", userID)
		})
	}

	if err := eg.Wait(); err != nil {
		return FullUser{}, err
	}

	for i := range stats.ModeStats {
		modeStats := &stats.ModeStats[i]
		lbRank, ok := lbRanks[modeStats.Mode.String()]
		if !ok {
			slog.WarnContext(ctx, "missing leaderboard rank for full user", "mode", modeStats.Mode)
			continue
		}
		modeStats.Rank = lbRank.Rank
	}

	if replayList == nil {
		replayList = []domain.FullReplay{}
	}
	return FullUser{User: user, Stats: stats, ReplayList: replayList}, nil
}

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
