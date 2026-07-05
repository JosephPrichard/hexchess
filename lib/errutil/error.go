package errutil

import (
	"errors"
)

func IsType[T error](err error) bool {
	var t T
	return errors.As(err, &t)
}

func LeafError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}
