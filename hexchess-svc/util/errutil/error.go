package errutil

import "fmt"

func Guardf(err error, format string, a ...any) error {
	if err == nil {
		return nil
	}
	a = append(a, err)
	return fmt.Errorf(format+": %w", a...)
}
