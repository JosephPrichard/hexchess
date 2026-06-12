package serrors

import (
	"errors"
	"fmt"
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

	if len(values) == 0 {
		return &ServiceError{Err: err}
	}

	v := map[string]any{"error": err}

	for i := 0; i+1 < len(values); i += 2 {
		valueStr, ok := values[i].(string)
		if !ok {
			valueStr = "!BADKEY"
		}
		v[valueStr] = values[i+1]
	}

	err = fmt.Errorf("%s: %w", message, err)

	return &ServiceError{Err: err, Values: v}
}

func Flatten(err error, values *[]any) {
	var serr *ServiceError
	if errors.As(err, &serr) {
		for k, v := range serr.Values {
			*values = append(*values, k, v)
		}
		Flatten(serr.Err, values)
	}
}
