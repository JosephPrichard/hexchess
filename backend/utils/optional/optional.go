package optional

import (
	"bytes"
	"encoding/json"
)

type Maybe[T any] struct {
	Value   T
	Present bool
}

var jsonNull = []byte("null")

// MarshalJSON serializes to null if not present, otherwise the value.
func (m Maybe[T]) MarshalJSON() ([]byte, error) {
	if !m.Present {
		return jsonNull, nil
	}
	return json.Marshal(m.Value)
}

// UnmarshalJSON sets Present=false on null, otherwise decodes the value.
func (m *Maybe[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		m.Present = false
		return nil
	}
	if err := json.Unmarshal(data, &m.Value); err != nil {
		return err
	}
	m.Present = true
	return nil
}

func (o Maybe[T]) OrElse(def T) T {
	if o.Present {
		return o.Value
	}
	return def
}

func Map[X, Y any](o Maybe[X], f func(X) Y) Maybe[Y] {
	if o.Present {
		return Just(f(o.Value))
	}
	return Nothing[Y]()
}

func New[T any](v T, b bool) Maybe[T] {
	return Maybe[T]{Value: v, Present: b}
}

func Just[T any](v T) Maybe[T] {
	return Maybe[T]{Value: v, Present: true}
}

func Nothing[T any]() Maybe[T] {
	return Maybe[T]{}
}
