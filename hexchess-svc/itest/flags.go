package itest

type TestFlag int

const (
	RWPostgres TestFlag = iota
	ROPostgres
	Redis
)
