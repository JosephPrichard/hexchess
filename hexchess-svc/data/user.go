package data

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"log/slog"
	"math"
	"sort"
	"time"
)

type UserEntity struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	Country    string    `json:"country"`
	Elo        float64   `json:"elo"`
	HighestElo float64   `json:"highestElo"`
	Wins       int32     `json:"wins"`
	Losses     int32     `json:"losses"`
	Rank       int64     `json:"rank"`
	Bio        string    `json:"bio"`
	JoinedOn   time.Time `json:"joinedOn"`
	Total      int64     `json:"total"`
	WinRate    int64     `json:"winRate"`
}

var UserEntityCmpOpts = cmpopts.IgnoreFields(UserEntity{}, "JoinedOn")

const StartElo float64 = 1000

type RankedUser struct {
	ID   int64
	Rank int64
}

var ErrUserNotFound = errors.New("user not found")

func JoinRanks(rankedUsers []RankedUser, users []UserEntity) error {
	for i := range users {
		user := &users[i]
		found := false
		for _, rankedUser := range rankedUsers {
			if rankedUser.ID == user.ID {
				user.Rank = rankedUser.Rank
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %d not found in ranked users", user.ID)
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Rank < users[j].Rank
	})
	return nil
}

var ErrTakenUsername = errors.New("username already taken")

type UserInst struct {
	Username string  `json:"username"`
	Password string  `json:"password"`
	Country  string  `json:"country"`
	Elo      float64 `json:"elo"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
}

type HashResult struct {
	Salt           string
	HashedPassword string
}

func hashPassword(password string) (HashResult, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return HashResult{}, fmt.Errorf("failed to generate salt: %w", err)
	}
	salt := base64.StdEncoding.EncodeToString(saltBytes)

	saltedPassword := []byte(password + salt)

	hashed, err := bcrypt.GenerateFromPassword(saltedPassword, 12)
	if err != nil {
		return HashResult{}, fmt.Errorf("failed to hash password: %w", err)
	}

	return HashResult{Salt: salt, HashedPassword: string(hashed)}, nil
}

func mapInsertUserParams(inst UserInst, hash HashResult) db.InsertUserParams {
	return db.InsertUserParams{
		Username:   inst.Username,
		Country:    pgtype.Text{Valid: true, String: inst.Country},
		Elo:        inst.Elo,
		HighestElo: inst.Elo,
		Wins:       int32(inst.Wins),
		Losses:     int32(inst.Losses),
		Password:   hash.HashedPassword,
		Salt:       hash.Salt,
	}
}

func winrate(wins int32, total int32) int64 {
	wr := float64(0)
	if total > 0 {
		wr = float64(wins) / float64(total) * 100.0
	}
	return int64(wr)
}

func mapUserFromRow(row db.SelectUserByIDRow) UserEntity {
	total := row.Wins + row.Losses
	return UserEntity{
		ID:         row.ID,
		Username:   row.Username,
		Country:    row.Country.String,
		Elo:        row.Elo,
		HighestElo: row.HighestElo,
		Wins:       row.Wins,
		Losses:     row.Losses,
		Bio:        row.Bio,
		JoinedOn:   row.JoinedOn.Time,
		WinRate:    winrate(row.Wins, total),
		Total:      int64(total),
	}
}

func InsertUser(ctx context.Context, q *db.Queries, inst UserInst) (UserEntity, error) {
	fail := func(str string, err error) (UserEntity, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.ErrorContext(ctx, "failed to insert user", "inst", inst, "err", err)
		return UserEntity{}, err
	}

	hash, err := hashPassword(inst.Password)
	if err != nil {
		return fail("failed to generate hash", err)
	}

	row, err := q.InsertUser(ctx, mapInsertUserParams(inst, hash))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			slog.InfoContext(ctx, "player already exists", "inst", inst, "err", pgErr)
			return UserEntity{}, ErrTakenUsername
		}
	}
	if err != nil {
		return fail("failed to insert user to db", err)
	}

	user := mapUserFromRow(db.SelectUserByIDRow(row))
	slog.InfoContext(ctx, "created a new user", "user", user)
	return user, nil
}

func BatchInsertUsers(ctx context.Context, q *db.Queries, insts []UserInst) ([]UserEntity, error) {
	batches := make([]db.BatchInsertUserParams, len(insts))

	var eg errgroup.Group
	for i, inst := range insts {
		eg.Go(func() error {
			hash, err := hashPassword(inst.Password)
			if err != nil {
				return fmt.Errorf("failed to hash password for inst index %d: %w", i, err)
			}
			batch := mapInsertUserParams(inst, hash)
			batches[i] = db.BatchInsertUserParams(batch)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "failed to batch insert users", "insts", insts, "err", err)
		return nil, err
	}

	var users []UserEntity
	var errs []error

	q.BatchInsertUser(ctx, batches).QueryRow(func(i int, row db.BatchInsertUserRow, err error) {
		if err != nil {
			errs = append(errs, err)
		} else {
			users = append(users, mapUserFromRow(db.SelectUserByIDRow(row)))
		}
	})

	util.DynLog(ctx, "batch inserted user", errors.Join(errs...), "insts", insts, "users", users)
	return users, nil
}

type VerifiedUser struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Country  string  `json:"country"`
	Elo      float64 `json:"elo"`
}

func VerifyUserTx(ctx context.Context, pgDB PgDB, username string, inputPassword string) (VerifiedUser, error) {
	return WithTxn(TxnArgs[VerifiedUser]{
		Ctx:  ctx,
		PgDB: pgDB,
		TxFn: func(q *db.Queries) (VerifiedUser, error) {
			// we want to wrap this in a txn, so only a single login attempt can be active at a time - to prevent DDOS attacks used to skip login attempt count
			return VerifyUser(ctx, q, username, inputPassword)
		},
		ErrWhiteList: []error{ErrTooManyLoginAttempts, ErrUserNotFound},
	})
}

const LoginAttemptsDivisor = 10
const LockoutDuration = time.Minute * 1

var ErrTooManyLoginAttempts = errors.New("too many login attempts")

func VerifyUser(ctx context.Context, q *db.Queries, username string, inputPassword string) (VerifiedUser, error) {
	login, err := q.SelectLoginByName(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return VerifiedUser{}, ErrUserNotFound
	} else if err != nil {
		return VerifiedUser{}, fmt.Errorf("failed to select user '%s' by login: %w", username, err)
	}

	isExceedAttempts := login.LoginAttempts > 0 && login.LoginAttempts%LoginAttemptsDivisor == 0
	nextLoginTime := login.LastLoginAttempt.Time.Add(LockoutDuration)
	isLocked := isExceedAttempts && time.Now().Before(nextLoginTime)
	if isLocked {
		return VerifiedUser{}, ErrTooManyLoginAttempts
	}
	saltedPassword := inputPassword + login.Salt
	loginErr := bcrypt.CompareHashAndPassword([]byte(login.Password), []byte(saltedPassword))

	if loginErr == nil {
		if err := q.ResetLoginAttempts(ctx, login.ID); err != nil {
			return VerifiedUser{}, fmt.Errorf("failed to update user %d login attempts: %w", login.ID, err)
		}
		u := VerifiedUser{ID: login.ID, Username: login.Username, Country: login.Country.String, Elo: login.Elo}
		slog.InfoContext(ctx, "user login is valid", "user", u)
		return u, nil
	} else {
		if err := q.IncrLoginAttempts(ctx, login.ID); err != nil {
			return VerifiedUser{}, fmt.Errorf("failed to update user %d login attempts: %w", login.ID, err)
		}
		slog.ErrorContext(ctx, "user login is invalid", "username", username, "err", loginErr)
		return VerifiedUser{}, ErrUserNotFound
	}
}

func ProbabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}

type UpdtUserParams struct {
	Username string
	Bio      string
	Country  string
}

func UpdateUser(ctx context.Context, q *db.Queries, id int64, updt UpdtUserParams) (UserEntity, error) {
	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return UserEntity{}, nil
	}

	row, err := q.UpdateUser(ctx, db.UpdateUserParams{
		ID:       id,
		Username: pgtype.Text{Valid: updt.Username != "", String: updt.Username},
		Bio:      pgtype.Text{Valid: updt.Bio != "", String: updt.Bio},
		Country:  pgtype.Text{Valid: updt.Country != "", String: updt.Country},
	})

	user := mapUserFromRow(db.SelectUserByIDRow(row))
	util.DynLog(ctx, "updated user", err, "user", user)
	return user, err
}

func UpdateUserPassword(ctx context.Context, q *db.Queries, id int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password for user %d: %w", id, err)
	}
	err = q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: id, Password: hash.HashedPassword, Salt: hash.Salt})
	util.DynLog(ctx, "updated password", err, "id", id)
	return err
}

func GetUserByID(ctx context.Context, q *db.Queries, id int64) (UserEntity, error) {
	row, err := q.SelectUserByID(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, "failed to select user", "id", id, "err", err)
		return UserEntity{}, fmt.Errorf("failed to select user %d: %w", id, err)
	}
	user := mapUserFromRow(row)
	slog.InfoContext(ctx, "selected user", "id", id, "user", user)
	return user, nil
}

func GetRankedUsers(ctx context.Context, q *db.Queries, users []RankedUser) ([]UserEntity, error) {
	var ids []int64
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return GetUserByIDs(ctx, q, ids)
}

func GetUserByIDs(ctx context.Context, q *db.Queries, ids []int64) ([]UserEntity, error) {
	rows, err := q.SelectUsersByIDs(ctx, ids)
	if err != nil {
		slog.ErrorContext(ctx, "failed to select many users", "ids", ids, "err", err)
		return nil, fmt.Errorf("failed to select many users: %w", err)
	}
	var users []UserEntity
	for _, row := range rows {
		users = append(users, mapUserFromRow(db.SelectUserByIDRow(row)))
	}
	slog.InfoContext(ctx, "selected users", "ids", ids, "users", users)
	return users, nil
}

const MaxSearchOffset = 1000

var ErrSearchLimit = errors.New("search limit exceeded")

func SearchUsersByName(ctx context.Context, q *db.Queries, name string, page, perPage int32) ([]UserEntity, error) {
	page = max(page, 1)
	offset := (page - 1) * perPage
	if offset > MaxSearchOffset {
		slog.ErrorContext(ctx, "search offset exceeds maximum", "offset", offset, "maxOffset", MaxSearchOffset)
		return nil, ErrSearchLimit
	}

	rows, err := q.SelectUsersBySimilarity(ctx, db.SelectUsersBySimilarityParams{Username: name, Limit: perPage, Offset: offset})
	if err != nil {
		slog.ErrorContext(ctx, "failed to select users by similarity", "err", err, "name", name, "page", page, "limit", perPage, "offset", offset)
		return nil, fmt.Errorf("failed to select users by similarity: %w", err)
	}

	var users []UserEntity
	for i, row := range rows {
		rank := (page-1)*perPage + int32(i) + 1
		total := row.Wins + row.Losses
		users = append(users, UserEntity{
			ID:       row.ID,
			Username: row.Username,
			Country:  row.Country.String,
			Elo:      row.Elo,
			Wins:     row.Wins,
			Losses:   row.Losses,
			Rank:     int64(rank),
			Total:    int64(total),
			WinRate:  winrate(row.Wins, total),
		})
	}

	slog.InfoContext(ctx, "selected users by name similarity", "users", users, "name", name, "page", page, "limit", page, "offset", offset)
	return users, nil
}
