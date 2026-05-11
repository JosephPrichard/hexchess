package db

const (
	ErrPgUniqueViolation      = "23505"
	ErrPgForeignKeyViolation  = "23503"
	ErrPgCheckViolation       = "23506"
	ErrPgSerializationFailure = "40001"
	ErrPgDeadlock             = "40P01"
)
