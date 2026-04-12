package enum

import (
	"encoding/json"
	"fmt"
	"log/slog"
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

func ParseOk[T ~int, S StringLike](s S, m map[string]T) (T, bool) {
	v, ok := m[string(s)]
	return v, ok
}

func Expect[T ~int, S StringLike](s S, m map[string]T) T {
	v, err := Parse(s, m)
	if err != nil {
		slog.Error("failed to parse enum", "enum", fmt.Sprintf("%T", v), "err", err)
		panic(err.Error())
	}
	return v
}

type ParseError[T any] struct {
	Expected map[string]T
	Actual   string
}

func (err ParseError[T]) Error() string {
	return fmt.Sprintf("expected one of %v, got %v", err.Expected, err.Actual)
}
