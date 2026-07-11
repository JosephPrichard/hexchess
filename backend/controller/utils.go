package controller

import (
	"hexchess-svc/utils/enum"
	"hexchess-svc/utils/optional"
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

func parseOptFloat(q queryParseCtx, key string) optional.Maybe[float64] {
	v := q.Values.Get(key)
	if v == "" {
		return optional.Maybe[float64]{}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return optional.Just(f)
}

func parseOptString(q queryParseCtx, key string) optional.Maybe[string] {
	v := q.Values.Get(key)
	if v == "" {
		return optional.Maybe[string]{}
	}
	return optional.Just(v)
}

func parseOptDatetime(q queryParseCtx, key string) optional.Maybe[time.Time] {
	v := q.Values.Get(key)
	if v == "" {
		return optional.Maybe[time.Time]{}
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return optional.Just(t)
}

func parseOptInt[T interface{ int | int32 | int64 }](q queryParseCtx, key string) optional.Maybe[T] {
	v := q.Values.Get(key)
	if v == "" {
		return optional.Maybe[T]{}
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		q.RespErr.Put(key, err)
	}
	return optional.Just(T(i))
}

func parseOptEnum[T ~int](q queryParseCtx, key string, enums map[string]T) optional.Maybe[T] {
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
