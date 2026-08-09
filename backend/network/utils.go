package network

import (
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/opt"
	"net/url"
	"strconv"
	"time"
)

type queryParseCtx struct {
	Values  url.Values
	RespErr *BadRequestError
}

func makeQueryParseCtx(values url.Values) queryParseCtx {
	return queryParseCtx{Values: values, RespErr: &BadRequestError{}}
}

func parseInt(q queryParseCtx, key string) int {
	str := q.Values.Get(key)
	v, err := strconv.Atoi(str)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseDefaultInt[T interface{ int | int32 | int64 }](q queryParseCtx, key string, def T) T {
	str := q.Values.Get(key)
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return T(v)
}

func parseOptFloat(q queryParseCtx, key string) opt.Option[float64] {
	v := q.Values.Get(key)
	if v == "" {
		return opt.Option[float64]{}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return opt.Some(f)
}

func parseOptString(q queryParseCtx, key string) opt.Option[string] {
	v := q.Values.Get(key)
	if v == "" {
		return opt.Option[string]{}
	}
	return opt.Some(v)
}

func parseOptDatetime(q queryParseCtx, key string) opt.Option[time.Time] {
	v := q.Values.Get(key)
	if v == "" {
		return opt.Option[time.Time]{}
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return opt.Some(t)
}

func parseOptInt[T interface{ int | int32 | int64 }](q queryParseCtx, key string) opt.Option[T] {
	v := q.Values.Get(key)
	if v == "" {
		return opt.Option[T]{}
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return opt.Some(T(i))
}

func parseOptEnum[T ~int](q queryParseCtx, key string, enums map[string]T) opt.Option[T] {
	v, err := enum.ParseOptional(q.Values.Get(key), enums)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseEnum[T ~int](q queryParseCtx, key string, enums map[string]T) T {
	v, err := enum.Parse(q.Values.Get(key), enums)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}

func parseDefEnum[T ~int](q queryParseCtx, key string, enums map[string]T, def T) T {
	v, err := enum.ParseDefault(q.Values.Get(key), enums, def)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return v
}
