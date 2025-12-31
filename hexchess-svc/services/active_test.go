package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hexchess-svc/db"
	"hexchess-svc/outbound"
	"hexchess-svc/pkg/logutil"
	"testing"
	"time"
)

func TestActiveUser(t *testing.T) {
	// given
	rdb := db.BeforeRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, "testing-active-user")

	s1 := MakeActiveScenario()
	s2 := ActiveScenario{&outbound.StableGenerator{Time: time.UnixMilli(100)}, 100}
	s3 := ActiveScenario{&outbound.StableGenerator{Time: time.UnixMilli(1000)}, 0}

	// when
	_, err := s1.AddActiveUser(ctx, rdb, "1")
	require.NoError(t, err)
	countAfterAdding, err := s1.AddActiveUser(ctx, rdb, "2")
	require.NoError(t, err)

	countAfterRemoval, err := s1.RemoveActiveUser(ctx, rdb, "2")
	require.NoError(t, err)

	// adds and does not expire
	countAfterRemoveAndAdd, err := s2.AddActiveUser(ctx, rdb, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	countAfterExpiry, err := s3.GetActiveCount(ctx, rdb)
	require.NoError(t, err)

	// then
	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
