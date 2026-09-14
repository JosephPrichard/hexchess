package network_test

import (
	"errors"
	"hexchess-svc/network"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRespError_Put(t *testing.T) {
	var resp network.BadRequestError

	resp.Put("key1", errors.New("testing1"))
	resp.Put("key2", errors.New("testing2"))
	resp.Put("key3", errors.New("testing3"))

	assert.Equal(t, &network.BadRequestError{
		Errors: map[string]error{
			"key1": errors.New("testing1"),
			"key2": errors.New("testing2"),
			"key3": errors.New("testing3"),
		},
	}, &resp)
}
