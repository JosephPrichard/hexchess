package errutil

import (
	"errors"
)

func IsType[T error](err error) bool {
	var t T
	return errors.As(err, &t)
}
