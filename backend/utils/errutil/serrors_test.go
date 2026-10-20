package errutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err1 := New("doing thing 1", errors.New("leaf"), "key1", "value1")
	err2 := New("doing thing 2", err1, "key2", "value2")
	err3 := New("doing thing 3", err2, "key2", "value2")

	assert.NotNil(t, err3)
	assert.Equal(t, "doing thing 3: doing thing 2: doing thing 1: leaf", err3.Error())

	values := make(map[string]any)
	err3.(*ServiceError).Walk(values)

	assert.Equal(t, map[string]any{"key2": "value2", "key1": "value1"}, values)
}
