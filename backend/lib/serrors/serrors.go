package serrors

import (
	"fmt"
	"strings"
)

type SError struct {
	internal error
	Values   map[string]any
}

func (e SError) Error() string {
	return e.internal.Error()
}

func New(message string, err error) SError {
	return Format(message, err, nil)
}

func Format(message string, err error, values map[string]any) SError {
	if len(values) == 0 {
		return SError{internal: err}
	}

	var sb strings.Builder
	sb.WriteString(message)
	sb.WriteString(": ")
	sb.WriteString("%w")

	i := 0
	for k, v := range values {
		sb.WriteString(k)
		sb.WriteString(fmt.Sprintf("=%+v", v))
		if i < len(values)-1 {
			sb.WriteString(", ")
		}
		i++
	}

	werr := fmt.Errorf(sb.String(), err)
	return SError{internal: werr, Values: values}
}
