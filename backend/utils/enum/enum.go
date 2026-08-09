package enum

import (
	"encoding/json"
	"fmt"
	"hexchess-svc/utils/opt"
	"log/slog"
	"maps"
	"slices"
)

type StringLike interface {
	~string
}

type Entry[T ~int] struct {
	Enum   T
	String string
}

func BuildReverseMap[T ~int](entries []Entry[T]) map[string]T {
	m := make(map[string]T, len(entries))
	for _, e := range entries {
		m[e.String] = e.Enum
	}
	return m
}

func String[T ~int](val T, entries []Entry[T]) string {
	for _, e := range entries {
		if e.Enum == val {
			return e.String
		}
	}
	return "UNKNOWN"
}

func Marshal[T ~int](val T, entries []Entry[T]) ([]byte, error) {
	for _, e := range entries {
		if e.Enum == val {
			return json.Marshal(e.String)
		}
	}
	return nil, fmt.Errorf("unknown enum value: %d", val)
}

func Unmarshal[T ~int](data []byte, m map[string]T, out *T) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	e, ok := m[s]
	if !ok {
		return fmt.Errorf("unknown enum value: %q", s)
	}
	*out = e
	return nil
}

func Parse[T ~int, S StringLike](s S, m map[string]T) (T, error) {
	v, ok := m[string(s)]
	if !ok {
		return 0, ParseError[T]{Expected: m, Actual: string(s)}
	}
	return v, nil
}

func ParseDefault[T ~int, S StringLike](s S, m map[string]T, def T) (T, error) {
	if s == "" {
		return def, nil
	}
	return Parse(s, m)
}

func ParseOptional[T ~int, S StringLike](s S, m map[string]T) (opt.Option[T], error) {
	if s == "" {
		return opt.Option[T]{}, nil
	}
	v, ok := m[string(s)]
	if !ok {
		return opt.Option[T]{}, ParseError[T]{Expected: m, Actual: string(s)}
	}
	return opt.Some(v), nil
}

func Expect[T ~int, S StringLike](s S, m map[string]T) T {
	v, err := Parse(s, m)
	if err != nil {
		slog.Error("failed to parse enum", "enum", fmt.Sprintf("%T", v), "error", err)
		panic(err.Error())
	}
	return v
}

type ParseError[T any] struct {
	Expected map[string]T
	Actual   string
}

func (err ParseError[T]) Error() string {
	return fmt.Sprintf("expected one of %v, got '%v'", slices.Collect(maps.Keys(err.Expected)), err.Actual)
}
