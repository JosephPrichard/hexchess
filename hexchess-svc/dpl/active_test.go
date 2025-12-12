package dpl

import (
	"context"
	"hexchess-svc/infra"
	"hexchess-svc/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestActiveUser(t *testing.T) {
	rdb := infra.BeforeRedisTests(t)
	defer rdb.Close()

	ctx := context.WithValue(context.Background(), util.Trace, "testing-active-user")

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

	conn := rdb.Cache.Get()
	defer conn.Close()

	count, err = GetActiveCountWithExpiry(ctx, conn, rdb.ActiveUsersZSet, 1000)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
