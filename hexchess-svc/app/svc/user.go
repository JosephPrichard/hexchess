package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"hexchess-svc/db"
	"log/slog"
	"math"
	"sort"
	"time"
)

type UserEntity struct {
	ID         int64
	Username   string
	Country    string
	Elo        float64
	HighestElo float64
	Wins       int32
	Losses     int32
	Rank       int64
	Bio        string
	JoinedOn   time.Time
}

const StartElo float64 = 1000

type RankedUser struct {
	ID   int64
	Rank int64
}

var ErrUserNotFound = errors.New("user not found")

func JoinRanks(rankedList []RankedUser, userList []UserEntity) error {
	for i := range userList {
		user := &userList[i]
		found := false
		for _, rankedUser := range rankedList {
			if rankedUser.ID == user.ID {
				user.Rank = rankedUser.Rank
				found = true
				break
			}
		}
		if !found {
			return ErrUserNotFound
		}
	}

	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Rank < userList[j].Rank
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

func GenerateHash(password string) (HashResult, error) {
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

func mapUserFromRow(row db.SelectUserByIDRow) UserEntity {
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
	}
}

func InsertUser(ctx context.Context, q *db.Queries, inst UserInst) error {
	_, err := InsertUserRet(ctx, q, inst)
	return err
}

func InsertUserRet(ctx context.Context, q *db.Queries, inst UserInst) (UserEntity, error) {
	trace := ctx.Value(TraceKey)
	fail := func(str string, err error) (UserEntity, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to insert user", "inst", inst, "err", err, "trace", trace)
		return UserEntity{}, err
	}

	hash, err := GenerateHash(inst.Password)
	if err != nil {
		return fail("failed to generate hash", err)
	}

	row, err := q.InsertUser(ctx, mapInsertUserParams(inst, hash))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			slog.Info("player already exists", "inst", inst, "err", pgErr, "trace", trace)
			return UserEntity{}, ErrTakenUsername
		}
	}
	if err != nil {
		return fail("failed to insert user to db", err)
	}

	user := mapUserFromRow(db.SelectUserByIDRow(row))
	slog.Info("created a new user", "user", user, "trace", trace)
	return user, nil
}

func BatchInsertUsers(ctx context.Context, q *db.Queries, insts []UserInst) error {
	var batches []db.BatchInsertUserParams

	for _, inst := range insts {
		hash, err := GenerateHash(inst.Password)
		if err != nil {
			return err
		}
		batch := mapInsertUserParams(inst, hash)
		batches = append(batches, db.BatchInsertUserParams(batch))
	}

	rows, err := q.BatchInsertUser(ctx, batches)
	slog.Log(nil, dynLevel(err), "batch inserted users", "insts", insts, "err", err, "rowsAffected", rows, "trace", ctx.Value(TraceKey))
	return err
}

type VerifiedUser struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Country  string  `json:"country"`
	Elo      float64 `json:"elo"`
}

func VerifyUser(ctx context.Context, q *db.Queries, username string, inputPassword string) (VerifiedUser, error) {
	trace := ctx.Value(TraceKey)

	login, err := q.SelectLoginByName(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return VerifiedUser{}, ErrUserNotFound
	}
	if err != nil {
		return VerifiedUser{}, fmt.Errorf("failed to select user by name: %w", err)
	}
	saltedPassword := inputPassword + login.Salt
	if err := bcrypt.CompareHashAndPassword([]byte(login.Password), []byte(saltedPassword)); err != nil {
		slog.Info("user login is invalid", "username", username, "trace", trace)
		return VerifiedUser{}, ErrUserNotFound
	}

	u := VerifiedUser{ID: login.ID, Username: login.Username, Country: login.Country.String, Elo: login.Elo}
	slog.Info("user login is valid", "user", u, "trace", trace)
	return u, nil
}

type EloChangeSet struct {
	WinEloDiff  float64
	LoseEloDiff float64
}

func ProbabilityWins(elo1, elo2 float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (elo2-elo1)/400.0))
}

func UpdateUserStatsTx(ctx context.Context, pgDB DB, winID int64, loseID int64) (EloChangeSet, error) {
	return WithTransaction(ctx, pgDB, func(q *db.Queries) (EloChangeSet, error) {
		return UpdateUserStats(ctx, q, winID, loseID)
	})
}

func UpdateUserStats(ctx context.Context, q *db.Queries, winID int64, loseID int64) (EloChangeSet, error) {
	trace := ctx.Value(TraceKey)
	fail := func(str string, err error) (EloChangeSet, error) {
		err = fmt.Errorf("%s: %w", str, err)
		slog.Error("failed to update stats", "winID", winID, "loseID", loseID, "err", err, "trace", trace)
		return EloChangeSet{}, err
	}

	winElo, err := q.GetElo(ctx, winID)
	if err != nil {
		return fail("failed to get win elo", err)
	}
	loseElo, err := q.GetElo(ctx, loseID)
	if err != nil {
		return fail("failed to get lose elo", err)
	}

	winEloNext := winElo + 30*(1.0-ProbabilityWins(loseElo, winElo))
	loseEloNext := loseElo + (-30 * ProbabilityWins(winElo, loseElo))

	if err := q.UpdateWins(ctx, db.UpdateWinsParams{ID: winID, Elo: winEloNext}); err != nil {
		return fail("failed to update win elo", err)
	}
	if err := q.UpdateLosses(ctx, db.UpdateLossesParams{ID: loseID, Elo: loseEloNext}); err != nil {
		return fail("failed to update lose elo", err)
	}

	winEloDiff := winEloNext - winElo
	loseEloDiff := loseEloNext - loseElo

	slog.Info("updated user stats", "winId", winID, "loseId", loseID, "winEloDiff", winEloDiff, "loseEloDiff", loseEloDiff, "trace", trace)
	return EloChangeSet{WinEloDiff: winEloDiff, LoseEloDiff: loseEloDiff}, nil
}

func UpdateUser(ctx context.Context, q *db.Queries, id int64, newUsername string, newBio string, newCountry string) error {
	if newUsername == "" && newBio == "" && newCountry == "" {
		return nil
	}

	user, err := q.UpdateUser(ctx, db.UpdateUserParams{
		ID:       id,
		Username: pgtype.Text{Valid: newUsername != "", String: newUsername},
		Bio:      pgtype.Text{Valid: newBio != "", String: newBio},
		Country:  pgtype.Text{Valid: newCountry != "", String: newCountry},
	})

	slog.Log(nil, dynLevel(err), "updated user", "user", user, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func UpdateUserPassword(ctx context.Context, q *db.Queries, id int64, newPassword string) error {
	hash, err := GenerateHash(newPassword)
	if err != nil {
		return err
	}
	err = q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: id, Password: hash.HashedPassword, Salt: hash.Salt})
	slog.Log(nil, dynLevel(err), "updated password", "id", id, "err", err, "trace", ctx.Value(TraceKey))
	return err
}

func GetUserById(ctx context.Context, q *db.Queries, id int64) (UserEntity, error) {
	trace := ctx.Value(TraceKey)
	row, err := q.SelectUserByID(ctx, id)
	if err != nil {
		slog.Error("failed to select user", "id", id, "err", err, "trace", trace)
		return UserEntity{}, fmt.Errorf("failed to select user: %w", err)
	}
	user := mapUserFromRow(row)
	slog.Info("selected user", "id", id, "user", user, "trace", trace)
	return user, nil
}

func GetRankedUsers(ctx context.Context, q *db.Queries, users []RankedUser) ([]UserEntity, error) {
	var ids []int64
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return GetUserByIds(ctx, q, ids)
}

func GetUserByIds(ctx context.Context, q *db.Queries, ids []int64) ([]UserEntity, error) {
	trace := ctx.Value(TraceKey)
	rows, err := q.SelectUsersByIDs(ctx, ids)
	if err != nil {
		slog.Error("failed to select users", "ids", ids, "err", err, "trace", trace)
		return nil, fmt.Errorf("failed to select users: %w", err)
	}
	var users []UserEntity
	for _, row := range rows {
		users = append(users, mapUserFromRow(db.SelectUserByIDRow(row)))
	}
	slog.Info("selected users", "ids", ids, "users", users, "trace", trace)
	return users, nil
}

func SearchUsersByName(ctx context.Context, q *db.Queries, name string, page int32, perPage int32) ([]UserEntity, error) {
	trace := ctx.Value(TraceKey)

	page = max(page, 1)
	offset := (page - 1) * perPage

	rows, err := q.SelectUsersBySimilarity(ctx, db.SelectUsersBySimilarityParams{Username: name, Limit: perPage, Offset: offset})
	if err != nil {
		slog.Error("failed to select users by name", "err", err, "trace", trace)
		return nil, fmt.Errorf("failed to select users by name: %w", err)
	}

	var users []UserEntity
	for i, row := range rows {
		rank := (page-1)*perPage + int32(i) + 1
		user := UserEntity{
			ID:       row.ID,
			Username: row.Username,
			Country:  row.Country.String,
			Elo:      row.Elo,
			Wins:     row.Wins,
			Losses:   row.Losses,
			Rank:     int64(rank),
		}
		users = append(users, user)
	}

	slog.Info("selected users by name", "name", name, "page", page, "perPage", page, "offset", offset, "trace", trace)
	return users, nil
}
