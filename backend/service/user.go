package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"hexchess-svc/database/mutate"
	"hexchess-svc/model"
	"hexchess-svc/queue/producers"
	"hexchess-svc/utils/perf"
	"hexchess-svc/utils/serrors"
	"hexchess-svc/utils/slogutil"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"hexchess-svc/database"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

type UserService struct {
	// infra
	database.Database

	// service
	producer UpdateUserProducer
}

type UpdateUserProducer interface {
	ProduceUpdateUser(ctx context.Context, txn pgx.Tx, args producers.UpdateUserArgs) error
}

func NewUserService(database database.Database, producer UpdateUserProducer) *UserService {
	if producer == nil {
		producer = producers.NewRiverProducer(&producers.NoopRiverClient{})
	}
	return &UserService{Database: database, producer: producer}
}

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

func (service *UserService) InsertUser(ctx context.Context, inst UserInst) (user model.User, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	hash, err := hashPassword(inst.Password)
	if err != nil {
		return model.User{}, serrors.New("generate hash", err)
	}

	err = service.Database.ExecTx(ctx, database.TxArgs{
		// Transaction is required to make this transaction atomic, since there are no reads - any isolation level is okay
		Isolation:  pgx.ReadUncommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, txn pgx.Tx, query database.QuerierMutator) error {
			return insertUser(ctx, txn, query, service.producer, inst, hash, &user)
		},
	})

	return
}

func insertUser(
	ctx context.Context,
	txn pgx.Tx, query database.QuerierMutator, producer UpdateUserProducer,
	inst UserInst, hash HashResult, user *model.User,
) error {
	if inst.JoinedOn.IsZero() {
		inst.JoinedOn = time.Now()
	}

	userRow, err := query.InsertUser(ctx, mutate.InsertUserParams{
		Username: inst.Username,
		Country:  inst.Country,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
		JoinedOn: pgtype.Timestamptz{Valid: true, Time: inst.JoinedOn},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTakenUsername
		}
		return serrors.New("insert user", err)
	}

	producerArgs := producers.UpdateUserArgs{ID: userRow.ID}
	if err := producer.ProduceUpdateUser(ctx, txn, producerArgs); err != nil {
		return err
	}

	*user = model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}

	slog.InfoContext(ctx, "created a new user", "user", user)
	return nil
}

func (service *UserService) GetUserByID(ctx context.Context, id int64) (_ model.User, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	userRow, err := service.Querier.SelectUserByID(ctx, id)
	if database.IsErrNoRows(err) {
		return model.User{}, ErrUserNotFound
	} else if err != nil {
		return model.User{}, serrors.New("select user", err, "userID", id)
	}

	user := model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}

	slog.InfoContext(ctx, "selected user by id", "userID", id, "user", user)
	return user, nil
}

type UpdtUserParams struct {
	Username string
	Bio      string
	Country  string
}

func (service *UserService) UpdateUser(ctx context.Context, id int64, updt UpdtUserParams) (user model.User, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	err = service.Database.ExecTx(ctx, database.TxArgs{
		// Transaction is required to make this transaction atomic, since there are no reads - any isolation level is okay
		Isolation:  pgx.ReadUncommitted,
		RetryCount: 1,
		QueryFn: func(ctx context.Context, txn pgx.Tx, query database.QuerierMutator) error {
			return updateUser(ctx, txn, query, service.producer, id, updt, &user)
		},
	})

	return
}

func updateUser(
	ctx context.Context,
	txn pgx.Tx, query database.QuerierMutator, producer UpdateUserProducer,
	id int64, updt UpdtUserParams, user *model.User,
) error {
	if updt.Username == "" && updt.Bio == "" && updt.Country == "" {
		return nil
	}

	userRow, err := query.UpdateUser(ctx, mutate.UpdateUserParams{
		ID:       id,
		Username: database.OptString(updt.Username),
		Bio:      database.OptString(updt.Bio),
		Country:  database.OptString(updt.Country),
	})
	if err != nil {
		return serrors.New("update user", err, "userID", id)
	}

	producerArgs := producers.UpdateUserArgs{ID: userRow.ID}
	if err := producer.ProduceUpdateUser(ctx, txn, producerArgs); err != nil {
		return err
	}

	*user = model.User{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country, Bio: userRow.Bio, JoinedOn: userRow.JoinedOn.Time}

	slog.InfoContext(ctx, "updated user", "user", user)
	return err
}

func (service *UserService) UpdateUserPassword(ctx context.Context, id int64, newPassword string) (err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	hash, err := hashPassword(newPassword)
	if err != nil {
		return serrors.New("hash password for user", err, "userID", id)
	}

	err = service.QuerierMutator.UpdatePassword(ctx, mutate.UpdatePasswordParams{
		ID:       id,
		Password: hash.HashedPassword,
		Salt:     hash.Salt,
	})
	slog.Log(ctx, slogutil.ErrorLevel(err), "updated password", "error", err, "userID", id)
	return err
}

func (service *UserService) BatchInsertUsers(ctx context.Context, insts []UserInst) ([]model.User, error) {
	batches := make([]mutate.BatchInsertUserParams, len(insts))

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
			batch := mutate.BatchInsertUserParams{
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

	var rows []mutate.BatchInsertUserRow
	var batchErr error

	service.QuerierMutator.BatchInsertUser(ctx, batches).QueryRow(func(i int, row mutate.BatchInsertUserRow, err error) {
		rows = append(rows, row)
		batchErr = errors.Join(batchErr, err)
	})
	if batchErr != nil {
		slog.ErrorContext(ctx, "failed to batch inserted users", "err", batchErr)
		return nil, batchErr
	}

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

	slog.InfoContext(ctx, "batch inserted users", "users", users)
	return users, nil
}

type VerifiedUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Country  string `json:"country"`
}

const LoginAttemptsDivisor = 10
const LockoutDuration = time.Minute * 1

var ErrTooManyLoginAttempts = errors.New("too many login attempts")

func (service *UserService) VerifyUser(ctx context.Context, username string, inputPassword string) (user VerifiedUser, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	err = service.Database.ExecTx(ctx, database.TxArgs{
		// Serializable is required to prevent the following race conditions
		// Case 1 (Non-Repeatable Read):
		// T1 is allowed to log in due to valid login attempts L1 and increases login attempt count from L1 to L2
		// In between reading L1 and updating from L1 to L2, another query updates the login attempts, so L1 will return an inconsistent value
		// Case 2 (Write Skew):
		// T1 and T2 attempt to log in at the same time, with LoginAttempts-1 (L1) one less than an invalid threshold
		// T1 and T2 are both permitted to attempt to log in since L1 is valid, even though only one login attempt is allowed
		// L2 will be L1+2 after update, even though it should not have permitted both attempts
		Isolation:    pgx.Serializable,
		ErrAllowlist: []error{ErrTooManyLoginAttempts, ErrUserNotFound},
		// RetryCount: retry with hashing can be expensive, so keep this count at 1
		// if the same user tries multiple to log in attempts at once, one will fail - we map this to an auth error
		RetryCount: 1,
		QueryFn: func(ctx context.Context, _ pgx.Tx, query database.QuerierMutator) error {
			return verifyUser(ctx, query, username, inputPassword, &user)
		},
	})

	if database.IsSerializationFailure(err) {
		slog.ErrorContext(ctx, "while verifying user: transaction failure - override with login error", "error", err)
		err = ErrUserNotFound
	}

	return
}

func verifyUser(ctx context.Context, query database.QuerierMutator, username string, inputPassword string, user *VerifiedUser) error {
	loginRow, hashErr := query.SelectLoginByName(ctx, username)
	if database.IsErrNoRows(hashErr) {
		return ErrUserNotFound
	} else if hashErr != nil {
		return serrors.New("select user by login", hashErr, "username", username)
	}

	isExceedAttempts := loginRow.LoginAttempts > 0 && loginRow.LoginAttempts%LoginAttemptsDivisor == 0
	nextLoginTime := loginRow.LastLoginAttempt.Time.Add(LockoutDuration)
	isLocked := isExceedAttempts && time.Now().Before(nextLoginTime)
	if isLocked {
		return ErrTooManyLoginAttempts
	}

	saltedPassword := inputPassword + loginRow.Salt
	hashErr = bcrypt.CompareHashAndPassword([]byte(loginRow.Password), []byte(saltedPassword))

	if hashErr != nil {
		slog.WarnContext(ctx, "failed to login, credentials are invalid", "username", username, "error", hashErr)
	}

	if hashErr == nil {
		if err := query.ResetLoginAttempts(ctx, loginRow.ID); err != nil {
			return serrors.New("reset user login attempts", err, "userID", loginRow.ID)
		}
	} else {
		if err := query.IncrLoginAttempts(ctx, loginRow.ID); err != nil {
			return serrors.New("increment user login attempts", err, "userID", loginRow.ID)
		}
		return ErrUserNotFound
	}

	*user = VerifiedUser{ID: loginRow.ID, Username: loginRow.Username, Country: loginRow.Country}

	slog.InfoContext(ctx, "user login is valid", "user", user)
	return nil
}

type GoogleUserInst struct {
	Username string
	Country  string
	JoinedOn time.Time
}

func (service *UserService) SelectOrInsertGoogleUser(ctx context.Context, googleAccountID string, googleInst GoogleUserInst) (_ VerifiedUser, err error) {
	defer perf.WithContext(ctx).LogFailable(&err)

	var verifiedUser VerifiedUser
	var isCreated bool

	login, err := service.Querier.SelectByGoogleAccountID(ctx, pgtype.Text{String: googleAccountID, Valid: true})
	if database.IsErrNoRows(err) {
		isCreated = false
	} else if err != nil {
		return verifiedUser, serrors.New("select user by google account id", err, "googleAccountID", googleAccountID)
	} else {
		isCreated = true
	}

	if !isCreated {
		userRow, err := service.QuerierMutator.InsertUser(ctx, mutate.InsertUserParams{
			Username:        googleInst.Username,
			Country:         googleInst.Country,
			JoinedOn:        pgtype.Timestamptz{Time: googleInst.JoinedOn, Valid: true},
			GoogleAccountID: pgtype.Text{String: googleAccountID, Valid: true},
		})
		if err != nil {
			return verifiedUser, serrors.New("insert google user", err, "googleAccountID", googleAccountID)
		}

		slog.InfoContext(ctx, "inserted a google user account", "inst", googleInst, "googleAccountID", googleAccountID)

		verifiedUser = VerifiedUser{ID: userRow.ID, Username: userRow.Username, Country: userRow.Country}
	} else {
		verifiedUser = VerifiedUser{ID: login.ID, Username: login.Username, Country: login.Country}
	}

	slog.InfoContext(ctx, "resolved verified user from googleAccountID", "user", verifiedUser, "googleAccountID", googleAccountID)
	return verifiedUser, nil
}

type HashResult struct {
	Salt           string
	HashedPassword string
}

func hashPassword(password string) (HashResult, error) {
	defer perf.New().Log()

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
