package svc

import (
	"context"
	"github.com/stretchr/testify/require"
	"hexchess-svc/db"
	"hexchess-svc/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestActiveUser(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), util.Trace, "testing-active-user")

	// when
	_, err := AddActiveUser(ctx, rdb, "1")
	require.NoError(t, err)
	countAfterAdding, err := AddActiveUser(ctx, rdb, "2")
	require.NoError(t, err)

	_, err = RemoveActiveUser(ctx, rdb, "2")
	require.NoError(t, err)
	countAfterRemoveAndAdd, err := AddActiveUserOn(ctx, rdb, "3", time.UnixMilli(100), 0)
	require.NoError(t, err)

	countAfterExpiry, err := GetActiveCountWithExpiry(ctx, rdb, rdb.ActiveUsersZSet, 1000)
	require.NoError(t, err)

	// then
	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
