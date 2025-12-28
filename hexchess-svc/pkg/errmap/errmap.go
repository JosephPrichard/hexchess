package errmap

import "fmt"

// ErrorMap is a utility for storing errors for each field in a request body in a map, inspired by multierror
type ErrorMap struct {
	Errors map[string]error
}

func Put(left error, key string, right error) error {
	if right == nil {
		return left
	}
	if left == nil {
		left = &ErrorMap{}
	}
	var errm, ok = left.(*ErrorMap)
	if ok {
		if errm.Errors == nil {
			errm.Errors = make(map[string]error)
		}
		errm.Errors[key] = right
	}
	return left
}

func (m *ErrorMap) Error() string {
	return fmt.Sprintf("%+v", m.Errors)
}
