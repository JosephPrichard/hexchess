package errutil

import (
	"errors"
	"fmt"
	"maps"
)

type ServiceError struct {
	Err    error
	Values map[string]any
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

func (e *ServiceError) Error() string {
	return e.Err.Error()
}

func New(message string, err error, values ...any) error {
	if err == nil {
		return nil
	}
	err = fmt.Errorf("%s: %w", message, err)

	if len(values) == 0 {
		return &ServiceError{Err: err}
	}

	valuesMap := map[string]any{}

	for i := 0; i+1 < len(values); i += 2 {
		key := fmt.Sprintf("%s", values[i])
		valuesMap[key] = values[i+1]
	}

	return &ServiceError{Err: err, Values: valuesMap}
}

func (e *ServiceError) Walk(values map[string]any) {
	var err error = e
	for {
		if serviceError, ok := errors.AsType[*ServiceError](err); ok {
			maps.Copy(values, serviceError.Values)
			err = serviceError.Err
		} else {
			break
		}
	}
}
