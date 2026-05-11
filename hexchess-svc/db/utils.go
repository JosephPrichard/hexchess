package db

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"hexchess-svc/db/sqlc"
	"hexchess-svc/model"
	"hexchess-svc/util/enum"
	"time"
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

func MapOptInt8(o enum.Optional[int64]) pgtype.Int8 {
	return pgtype.Int8{Int64: o.Value, Valid: o.IsPresent}
}

func MapOptTime(o enum.Optional[time.Time]) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: o.Value, Valid: o.IsPresent}
}

func MapOptMode(o enum.Optional[model.GameMode]) sqlc.NullModeEnum {
	return sqlc.NullModeEnum{ModeEnum: sqlc.ModeEnum(o.Value.String()), Valid: o.IsPresent}
}

func MapOptResult(o enum.Optional[model.ReplayResult]) sqlc.NullResultEnum {
	return sqlc.NullResultEnum{ResultEnum: sqlc.ResultEnum(o.Value.String()), Valid: o.IsPresent}
}

func MapOptCause(o enum.Optional[model.ReplayCause]) sqlc.NullCauseEnum {
	return sqlc.NullCauseEnum{CauseEnum: sqlc.CauseEnum(o.Value.String()), Valid: o.IsPresent}
}
