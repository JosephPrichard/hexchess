package replay

import (
	"hexchess-svc/database/query"
	"hexchess-svc/utils/opt"
	"time"
)

func MapOptResult(o opt.Option[ReplayResult]) query.NullResultEnum {
	return query.NullResultEnum{ResultEnum: query.ResultEnum(o.Value.String()), Valid: o.Present}
}

func MapOptCause(o opt.Option[ReplayCause]) query.NullCauseEnum {
	return query.NullCauseEnum{CauseEnum: query.CauseEnum(o.Value.String()), Valid: o.Present}
}

func toDayEpoch(optTime opt.Option[time.Time]) opt.Option[int32] {
	// uses the unix epoch in days for range queries on date. this truncates away timestamp precision regarding hours, seconds, etc.
	return opt.Option[int32]{Value: int32(optTime.Value.Unix() / 86400), Present: optTime.Present}
}
