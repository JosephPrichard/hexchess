package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRespError_Put(t *testing.T) {
	var resp ResponseError

	resp.Put("key1", errors.New("testing1"))
	resp.Put("key2", errors.New("testing2"))
	resp.Put("key3", errors.New("testing3"))

	assert.Equal(t, &ResponseError{
		Errors: map[string]error{
			"key1": errors.New("testing1"),
			"key2": errors.New("testing2"),
			"key3": errors.New("testing3"),
		},
	}, &resp)
}
