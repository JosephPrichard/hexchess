package gamedata

import (
	"hexchess-svc/database/query"
	"hexchess-svc/utils/opt"
)

func MapOptMode(o opt.Option[GameMode]) query.NullModeEnum {
	return query.NullModeEnum{ModeEnum: query.ModeEnum(o.Value.String()), Valid: o.Present}
}
