package svc

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hexchess-svc/ext"
	"hexchess-svc/itest"

	"time"

	"hexchess-svc/pkg/logutil"
	"testing"
)

func TestActiveUser(t *testing.T) {
	// given
	rdb := itest.SetupRedisTest(t)
	defer rdb.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	s := State{Redis: rdb, EntropySource: &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 5))}}

	//s1 := MakeActiveScenario()
	//s2 := ActiveScenario{&ext.StableSource{Time: time.UnixMilli(100)}, 100}
	//s3 := ActiveScenario{&ext.StableSource{Time: time.UnixMilli(1000)}, 0}

	// when
	_, err := s.AddActiveUser(ctx, "1")
	require.NoError(t, err)
	countAfterAdding, err := s.AddActiveUser(ctx, "2")
	require.NoError(t, err)

	countAfterRemoval, err := s.RemoveActiveUser(ctx, "2")
	require.NoError(t, err)

	// adds and does not expire
	s.EntropySource = &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 2))}
	countAfterRemoveAndAdd, err := s.AddActiveUser(ctx, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	s.EntropySource = &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 3))}
	countAfterExpiry, err := s.GetActiveCount(ctx)
	require.NoError(t, err)

	// then
	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
