package errutil

import "fmt"

func Guardf(format string, err error, a ...any) error {
	if err == nil {
		return nil
	}
	a = append(a, err)
	return fmt.Errorf(format + ": %w", a...)
}