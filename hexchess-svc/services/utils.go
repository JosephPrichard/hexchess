package svc

func New[T any](v T) *T {
	return &v
}
