package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"hexchess-svc/lib/enum"
	"hexchess-svc/lib/serrors"
	"hexchess-svc/model"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"hexchess-svc/db"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/lib/logutil"

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

func (services *HexchessServices) InsertUser(ctx context.Context, inst UserInst) (model.User, error) {
	if inst.JoinedOn.IsZero() {
		inst.JoinedOn = time.Now()
	}

	hash, err := hashPassword(inst.Password)
	if err != nil {
		return model.User{}, serrors.New("generate hash", err)
	}

	userRow, err := services.querier.InsertUser(ctx, sqlc.InsertUserParams{
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

func (services *HexchessServices) BatchInsertUsers(ctx context.Context, insts []UserInst) ([]model.User, error) {
	batches := make([]sqlc.BatchInsertUserParams, len(insts))

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

	slog.InfoContext(ctx, "batch inserting users", "insts", insts)

	var rows []sqlc.BatchInsertUserRow
	var insertErrs []error
	services.querier.BatchInsertUser(ctx, batches).QueryRow(func(i int, row sqlc.BatchInsertUserRow, err error) {
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

	logutil.Log(ctx, "batch inserted users", err, "users", users)
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

func (services *HexchessServices) VerifyUser(ctx context.Context, username string, inputPassword string) (VerifiedUser, error) {
	var user VerifiedUser

	err := services.transactor.ExecTx(ctx, db.TxArgs{
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
		QueryFn: func(ctx context.Context, querier sqlc.Querier) error {
			loginRow, err := querier.SelectLoginByName(ctx, username)
			if db.IsErrNoRows(err) {
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
			loginErr := bcrypt.CompareHashAndPassword([]byte(loginRow.Password), []byte(saltedPassword))

			if loginErr != nil {
				if err := querier.IncrLoginAttempts(ctx, loginRow.ID); err != nil {
					return serrors.New("increment user login attempts", err, "userID", loginRow.ID)
				}
				slog.ErrorContext(ctx, "failed to login, credentials are invalid", "username", username, "error", loginErr)
				return ErrUserNotFound
			}

			if err := querier.ResetLoginAttempts(ctx, loginRow.ID); err != nil {
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

func (services *HexchessServices) SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (VerifiedUser, error) {
	var verifiedUser VerifiedUser
	var isCreated bool

	login, err := services.querier.SelectByGoogleAccountID(ctx, pgtype.Text{String: googleAccountID, Valid: true})
	if db.IsErrNoRows(err) {
		isCreated = false
	} else if err != nil {
		return verifiedUser, serrors.New("select user by google account id", err, "googleAccountID", googleAccountID)
	} else {
		isCreated = true
	}

	if !isCreated {
		userRow, err := services.querier.InsertUser(ctx, sqlc.InsertUserParams{
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

func (services *HexchessServices) UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (model.User, error) {
	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return model.User{}, nil
	}

	userRow, err := services.querier.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:       id,
		Username: db.OptString(updt.Username),
		Bio:      db.OptString(updt.Bio),
		Country:  db.OptString(updt.Country),
	})
	if err != nil {
		return model.User{}, serrors.New("update user", err, "userID", id)
	}

	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "updated user", "user", user)
	return user, err
}

func (services *HexchessServices) UpdateUserPassword(ctx context.Context, id int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return serrors.New("hash password for user", err, "userID", id)
	}
	err = services.querier.UpdatePassword(ctx, sqlc.UpdatePasswordParams{
		ID:       id,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
	})
	logutil.Log(ctx, "updated password", err, "userID", id)
	return err
}

func (services *HexchessServices) GetUserByID(ctx context.Context, id int64) (model.User, error) {
	userRow, err := services.querier.SelectUserByID(ctx, id)
	if db.IsErrNoRows(err) {
		return model.User{}, ErrUserNotFound
	} else if err != nil {
		return model.User{}, serrors.New("select user", err, "userID", id)
	}
	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}
	slog.InfoContext(ctx, "selected user", "userID", id, "user", user)
	return user, nil
}

func avg[T constraints.Integer | constraints.Float](currAvg T, currCount int, nextValue T) T {
	return (currAvg*T(currCount) + nextValue) / T(currCount+1)
}

func (services *HexchessServices) GetUserStats(ctx context.Context, id int64) (model.UserStats, error) {
	modeEloRows, err := services.querier.SelectUserElosByID(ctx, id)
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
		stats.AvgElo = avg(stats.AvgElo, len(stats.ModeStats), modeStats.Elo)
		stats.TotalWinrate = avg(stats.TotalWinrate, len(stats.ModeStats), modeStats.Winrate)

		stats.ModeStats = append(stats.ModeStats, modeStats)
	}

	slog.InfoContext(ctx, "selected user elos", "stats", stats)
	return stats, nil
}

type FullUser struct {
	User       model.User         `json:"user"`
	Stats      model.UserStats    `json:"stats"`
	ReplayList []model.FullReplay `json:"replayList"`
}

func (services *HexchessServices) GetFullUser(ctx context.Context, userID int64, perPage int32) (FullUser, error) {
	var user model.User
	var stats model.UserStats
	var replayList []model.FullReplay
	var lbRanks map[string]LbRank

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		user, err = services.GetUserByID(egCtx, userID)
		return serrors.New("get user", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		stats, err = services.GetUserStats(egCtx, userID)
		return serrors.New("get user stats", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		lbRanks, err = services.GetUserLeaderboardRanks(egCtx, userID, model.GameModeEnums)
		return serrors.New("get user leaderboard ranks", err, "userID", userID)
	})

	eg.Go(func() (err error) {
		replayList, err = services.SearchReplaysByQuery(egCtx, ReplaysQuery{
			UserID:  enum.Just(userID),
			PerPage: perPage,
		})
		return serrors.New("get user replays", err, "userID", userID)
	})

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
		replayList = []model.FullReplay{}
	}
	return FullUser{User: user, Stats: stats, ReplayList: replayList}, nil
}

func (services *HexchessServices) selectUsersByIDs(ctx context.Context, ids []int64) ([]model.User, error) {
	userRows, err := services.querier.SelectUsersByIDs(ctx, ids)

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

type UserIDByNameRequest struct {
	Username enum.Optional[string]
	SupplyID func(int64)
}

func (services *HexchessServices) getUserIDsByUsernames(ctx context.Context, requests []UserIDByNameRequest) error {
	var usernames []string
	for _, request := range requests {
		if !request.Username.IsPresent {
			continue
		}
		usernames = append(usernames, request.Username.Value)
	}
	if len(usernames) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "selecting user ids by usernames for requests", "requests", requests)

	userRows, err := services.querier.SelectUserIDsByNames(ctx, usernames)
	if err != nil {
		return serrors.New("select user ids by names", err, "usernames", usernames)
	}

	userIDs := make(map[string]int64)
	for _, row := range userRows {
		userIDs[row.Username] = row.ID
	}

	for _, request := range requests {
		if !request.Username.IsPresent {
			continue
		}
		userID, exists := userIDs[request.Username.Value]
		if !exists {
			return ErrUserNotFound
		}
		request.SupplyID(userID)
	}
	return nil
}
