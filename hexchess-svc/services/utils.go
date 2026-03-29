package svc

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
)

func ptr[T any](v T) *T {
	return &v
}

func IsErrNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}
