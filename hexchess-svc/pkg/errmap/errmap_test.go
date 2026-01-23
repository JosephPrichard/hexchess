package errmap

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestErrorMap_Put(t *testing.T) {
	errm := Put(nil, "key1", errors.New("testing1"))
	errm = Put(errm, "key2", errors.New("testing2"))
	errm = Put(errm, "key3", errors.New("testing3"))

	assert.Equal(t, &ErrorMap{
		Errors: map[string]error{
			"key1": errors.New("testing1"),
			"key2": errors.New("testing2"),
			"key3": errors.New("testing3"),
		},
	}, errm)
}
