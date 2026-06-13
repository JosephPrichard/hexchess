package serrors

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	err1 := New("doing thing 1", errors.New("leaf"), "key1", "value1")
	err2 := New("doing thing 2", err1, "key2", "value2")
	err3 := New("doing thing 3", err2, "key2", "value2")

	assert.NotNil(t, err3)
	assert.Equal(t, "doing thing 3: doing thing 2: doing thing 1: leaf", err3.Error())

	var values []any
	WalkValues(err3, &values)

	assert.Equal(t, []any{"key2", "value2", "key1", "value1"}, values)
}
