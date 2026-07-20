package db

import (
	"database/sql"
	"errors"
	"hexchess-svc/db/primarydb"
	"hexchess-svc/model"
	"hexchess-svc/utils/optional"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func IsErrNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}

func OptInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

func OptString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func OptTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

func OptBool(v bool) pgtype.Bool {
	return pgtype.Bool{Bool: v}
}

func MapOptInt8(o optional.Maybe[int64]) pgtype.Int8 {
	return pgtype.Int8{Int64: o.Value, Valid: o.Present}
}

func MapOptInt4(o optional.Maybe[int32]) pgtype.Int4 {
	return pgtype.Int4{Int32: o.Value, Valid: o.Present}
}

func MapOptTime(o optional.Maybe[time.Time]) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: o.Value, Valid: o.Present}
}

func MapOptMode(o optional.Maybe[model.GameMode]) primarydb.NullModeEnum {
	return primarydb.NullModeEnum{ModeEnum: primarydb.ModeEnum(o.Value.String()), Valid: o.Present}
}

func MapOptResult(o optional.Maybe[model.ReplayResult]) primarydb.NullResultEnum {
	return primarydb.NullResultEnum{ResultEnum: primarydb.ResultEnum(o.Value.String()), Valid: o.Present}
}

func MapOptCause(o optional.Maybe[model.ReplayCause]) primarydb.NullCauseEnum {
	return primarydb.NullCauseEnum{CauseEnum: primarydb.CauseEnum(o.Value.String()), Valid: o.Present}
}

func MapInsertErr(err error, uniqueViolationErr error, foreignKeyViolationErr error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Code {
	case ErrPgUniqueViolation:
		return uniqueViolationErr
	case ErrPgForeignKeyViolation, ErrPgCheckViolation:
		return foreignKeyViolationErr
	default:
		return nil
	}
}
