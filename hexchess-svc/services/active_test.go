package svc

import (
	"context"
	"testing"
	"time"

	"hexchess-svc/ext"
	"hexchess-svc/itest"
	"hexchess-svc/pkg/logutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveUser(t *testing.T) {
	// given
	services := SetupServicesTest(t, itest.Redis)
	defer services.Close()

	ctx := context.WithValue(t.Context(), logutil.Trace, t.Name())
	services.EntropySource = &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 5))}

	//s1 := MakeActiveScenario()
	//s2 := ActiveScenario{&ext.StableSource{Time: time.UnixMilli(100)}, 100}
	//s3 := ActiveScenario{&ext.StableSource{Time: time.UnixMilli(1000)}, 0}

	// when
	_, err := services.AddActiveUser(ctx, "1")
	require.NoError(t, err)
	countAfterAdding, err := services.AddActiveUser(ctx, "2")
	require.NoError(t, err)

	countAfterRemoval, err := services.RemoveActiveUser(ctx, "2")
	require.NoError(t, err)

	// adds and does not expire
	services.EntropySource = &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 2))}
	countAfterRemoveAndAdd, err := services.AddActiveUser(ctx, "3")
	require.NoError(t, err)

	// gets and expires the active user we just added, without expiring any others
	services.EntropySource = &ext.StableSource{Time: time.UnixMilli(int64(ActiveUserMaxage * 3))}
	countAfterExpiry, err := services.GetActiveCount(ctx)
	require.NoError(t, err)

	// then
	assert.Equal(t, int64(2), countAfterAdding)
	assert.Equal(t, int64(1), countAfterRemoval)
	assert.Equal(t, int64(2), countAfterRemoveAndAdd)
	assert.Equal(t, int64(1), countAfterExpiry)
}
