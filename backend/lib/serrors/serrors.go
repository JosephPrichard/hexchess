package serrors

import (
	"errors"
	"fmt"
	"slices"
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

func Wrap(message string, err error, values ...any) error {
	if err == nil {
		return nil
	}

	err = fmt.Errorf("%s: %w", message, err)

	if len(values) == 0 {
		return &ServiceError{Err: err}
	}

	v := map[string]any{}

	for i := 0; i+1 < len(values); i += 2 {
		valueStr, ok := values[i].(string)
		if !ok {
			valueStr = "!BADKEY"
		}
		v[valueStr] = values[i+1]
	}

	return &ServiceError{Err: err, Values: v}
}

func WalkValues(err error, values *[]any) {
	var serr *ServiceError
	for {
		if errors.As(err, &serr) {
			for k, v := range serr.Values {
				if slices.Contains(*values, any(k)) {
					continue
				}
				*values = append(*values, k, v)
			}
			err = serr.Err
		} else {
			break
		}
	}
}
