package serrors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err1 := Wrap("doing thing 1", errors.New("leaf"), "key1", "value1")
	err2 := Wrap("doing thing 2", err1, "key2", "value2")
	err3 := Wrap("doing thing 3", err2, "key2", "value2")

	assert.NotNil(t, err3)
	assert.Equal(t, "doing thing 3: doing thing 2: doing thing 1: leaf", err3.Error())

	var values []any
	WalkValues(err3, &values)

	assert.Equal(t, []any{"key2", "value2", "key1", "value1"}, values)
}
