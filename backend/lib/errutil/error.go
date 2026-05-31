package errutil

import (
	"errors"
	"fmt"
)

func Guardf(err error, format string, a ...any) error {
	if err == nil {
		return nil
	}
	a = append(a, err)
	return fmt.Errorf(format+": %w", a...)
}

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
