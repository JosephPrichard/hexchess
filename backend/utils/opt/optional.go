package opt

import (
	"bytes"
	"encoding/json"
)

type Option[T any] struct {
	Value   T
	Present bool
}

var jsonNull = []byte("null")

// MarshalJSON serializes to null if not present, otherwise the value.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.Present {
		return jsonNull, nil
	}
	return json.Marshal(o.Value)
}

// UnmarshalJSON sets Present=false on null, otherwise decodes the value.
func (o *Option[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		o.Present = false
		return nil
	}
	if err := json.Unmarshal(data, &o.Value); err != nil {
		return err
	}
	o.Present = true
	return nil
}

func New[T any](v T, b bool) Option[T] {
	return Option[T]{Value: v, Present: b}
}

func Some[T any](v T) Option[T] {
	return Option[T]{Value: v, Present: true}
}

func None[T any]() Option[T] {
	return Option[T]{}
}
