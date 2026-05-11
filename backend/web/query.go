package web

import (
	"fmt"
	"hexchess-svc/util/enum"
	"net/url"
	"strconv"
	"time"
)

type QueryParseCtx struct {
	Values  url.Values
	RespErr *ResponseError
}

func MakeQueryParseCtx(values url.Values) QueryParseCtx {
	return QueryParseCtx{Values: values, RespErr: &ResponseError{}}
}

func parseInt(ctx QueryParseCtx, key string) int {
	str := ctx.Values.Get(key)
	v, err := strconv.Atoi(str)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{fmt.Errorf("failed to parse integer: '%s'", str)})
	}
	return v
}

func parseDefaultInt[T interface{ int | int32 | int64 }](ctx QueryParseCtx, key string, def T) T {
	str := ctx.Values.Get(key)
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return T(v)
}

func parseOptFloat(ctx QueryParseCtx, key string) enum.Optional[float64] {
	v := ctx.Values.Get(key)
	if v == "" {
		return enum.Optional[float64]{}
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return enum.Just(f)
}

func parseDefaultString(values url.Values, key, def string) string {
	v := values.Get(key)
	if v == "" {
		return def
	}
	return v
}

func parseOptString(ctx QueryParseCtx, key string) enum.Optional[string] {
	v := ctx.Values.Get(key)
	if v == "" {
		return enum.Optional[string]{}
	}
	return enum.Just(v)
}

func parseOptDatetime(ctx QueryParseCtx, key string) enum.Optional[time.Time] {
	v := ctx.Values.Get(key)
	if v == "" {
		return enum.Optional[time.Time]{}
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return enum.Just(t)
}

func parseOptInt[T interface{ int | int32 | int64 }](ctx QueryParseCtx, key string) enum.Optional[T] {
	v := ctx.Values.Get(key)
	if v == "" {
		return enum.Optional[T]{}
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return enum.Just(T(i))
}

func parseOptEnum[T ~int](ctx QueryParseCtx, key string, enums map[string]T) enum.Optional[T] {
	v, err := enum.ParseOptional(ctx.Values.Get(key), enums)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return v
}

func parseEnum[T ~int](ctx QueryParseCtx, key string, enums map[string]T) T {
	v, err := enum.Parse(ctx.Values.Get(key), enums)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return v
}

func parseDefEnum[T ~int](ctx QueryParseCtx, key string, enums map[string]T, def T) T {
	v, err := enum.ParseDefault(ctx.Values.Get(key), enums, def)
	if err != nil {
		ctx.RespErr.Put(key, BadRequestError{err})
	}
	return v
}
