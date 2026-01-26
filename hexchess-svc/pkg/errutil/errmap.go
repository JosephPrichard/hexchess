package errutil

import "fmt"

// ErrorMap is a utility for storing errors for each field in a request body in a map, inspired by multierror
type ErrorMap struct {
	Errors map[string]error
}

func PutMap(err error, key string, newErr error) error {
	if newErr == nil {
		return err
	}
	if err == nil {
		err = &ErrorMap{}
	}
	var errm, ok = err.(*ErrorMap)
	if ok {
		if errm.Errors == nil {
			errm.Errors = make(map[string]error)
		}
		errm.Errors[key] = newErr
	}
	return err
}

func (m *ErrorMap) Error() string {
	return fmt.Sprintf("%+v", m.Errors)
}
