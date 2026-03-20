package itest

func new[T any](v T) *T {
	return &v
}