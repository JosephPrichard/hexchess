package svc

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db"
	"time"
)

func ptr[T any](v T) *T {
	return &v
}

func IsErrNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}

func mapInsertErr(err error, uniqueViolationErr error, foreignKeyViolationErr error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Code {
	case db.ErrPgUniqueViolation:
		return uniqueViolationErr
	case db.ErrPgForeignKeyViolation, db.ErrPgCheckViolation:
		return foreignKeyViolationErr
	default:
		return nil
	}
}

func optInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

func optTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}
