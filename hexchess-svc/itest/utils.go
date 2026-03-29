package itest

func ptr[T any](v T) *T {
	return &v
}
