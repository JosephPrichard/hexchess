package optional

type Maybe[T any] struct {
	Value     T
	Present bool
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
