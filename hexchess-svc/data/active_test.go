package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/lib"
	"testing"
	"time"
)

func TestActiveUser(t *testing.T) {
	rdb := BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), lib.TK, "testing-active-user")

	_, err := AddActiveUser(ctx, rdb, "1")
	assert.NoError(t, err)
	count, err := AddActiveUser(ctx, rdb, "2")
	assert.NoError(t, err)

	assert.Equal(t, int64(2), count)

	_, err = RemoveActiveUser(ctx, rdb, "2")
	assert.NoError(t, err)
	count, err = AddActiveUserOn(ctx, rdb, "3", time.UnixMilli(100), 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	conn := rdb.Primary.Get()
	defer conn.Close()

	count, err = GetActiveCountWithExpiry(ctx, conn, rdb.ActiveUsersZSet, 1000)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
