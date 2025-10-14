package data

import (
	"context"
	"github.com/stretchr/testify/assert"
	"hexchess-svc/app/util"
	"testing"
	"time"
)

func TestActiveUser(t *testing.T) {
	rdb, closer := beforeRedisTests(t)
	defer closer()

	ctx := context.WithValue(context.Background(), util.TraceKey, "test-active-user")

	assert.NoError(t, AddActiveUser(ctx, rdb, "1"))
	assert.NoError(t, AddActiveUser(ctx, rdb, "2"))

	count, err := GetActiveCount(ctx, rdb)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.NoError(t, RemoveActiveUser(ctx, rdb, "2"))
	assert.NoError(t, AddActiveUserOn(ctx, rdb, "3", time.UnixMilli(100)))

	count, err = GetActiveCountWithExpiry(ctx, rdb, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	count, err = GetActiveCountWithExpiry(ctx, rdb, 1000)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
