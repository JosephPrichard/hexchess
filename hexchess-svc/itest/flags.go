package itest

type TestFlag int

const (
	UseTxn TestFlag = iota
	WithPostgres
	WithRedis
	WithAws
)
