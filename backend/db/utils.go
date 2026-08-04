package db

import (
	"database/sql"
	"errors"
	"hexchess-svc/db/query"
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

func MapOptInt8(o optional.Option[int64]) pgtype.Int8 {
	return pgtype.Int8{Int64: o.Value, Valid: o.Present}
}

func MapOptInt4(o optional.Option[int32]) pgtype.Int4 {
	return pgtype.Int4{Int32: o.Value, Valid: o.Present}
}

func MapOptTime(o optional.Option[time.Time]) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: o.Value, Valid: o.Present}
}

func MapOptMode(o optional.Option[model.GameMode]) query.NullModeEnum {
	return query.NullModeEnum{ModeEnum: query.ModeEnum(o.Value.String()), Valid: o.Present}
}

func MapOptResult(o optional.Option[model.ReplayResult]) query.NullResultEnum {
	return query.NullResultEnum{ResultEnum: query.ResultEnum(o.Value.String()), Valid: o.Present}
}

func MapOptCause(o optional.Option[model.ReplayCause]) query.NullCauseEnum {
	return query.NullCauseEnum{CauseEnum: query.CauseEnum(o.Value.String()), Valid: o.Present}
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
