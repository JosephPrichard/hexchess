package svc

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"hexchess-svc/db"
)

func ptr[T any](v T) *T {
	return &v
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
